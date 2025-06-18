/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/fwielstra/crntmetrics/domain"
	"github.com/fwielstra/crntmetrics/glclient"
	"github.com/fwielstra/crntmetrics/sqlite"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// if true, only runs the search queries and prints the results, does not persits the data
var dontPersist bool

// updateCmd represents the update command
func NewUpdateCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Runs the queries and adds them to the database",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			updateData(db)
		},
	}

	cmd.PersistentFlags().BoolVar(&dontPersist, "dontPersist", false, "Run queries but do not persist the results in the database")

	return cmd
}

// the set of queries to execute if update is true.
var queryPairs = []domain.QueryPair{
	{
		Name:      "primary-button-web",
		ProjectID: 62, // sitecore plus
		Old:       `class=\"btn btn-primary extension:html`,
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"primary\" extension:html`,
	},
	// Note that button searches specifically look for react-native-paper buttons.
	{
		Name:      "primary-button-app",
		ProjectID: 3202, // apps
		Old:       `<Button mode=\"contained\" extension:tsx`,
		Crnt:      `<Button variant=\"primary\" extension:tsx`,
	},
	{
		Name:      "secondary-button-web",
		ProjectID: 62,                                           // sitecore plus
		Old:       `class=\""btn btn-secondary" extension:html`, // note that there is one use case where the button type is dynamic
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"secondary\" extension:html`,
	},
	{
		Name:      "secondary-button-app",
		ProjectID: 3202, // apps
		Old:       `<Button mode=\"outlined\" extension:tsx`,
		Crnt:      `Button variant=\"secondary\" extension:tsx`,
	},
	{
		Name:      "tertiary-button",
		ProjectID: 62, // sitecore plus
		Old:       `class=\""btn btn-link" extension:html`,
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"tertiary\" extension:html`,
	},
	{
		Name:      "tertiary-button-app",
		ProjectID: 3202, // apps
		Old:       `<Button mode=\"text\" extension:tsx`,
		Crnt:      `Button variant=\"tertiary\" extension:tsx`,
	},
	{
		Name:      "icon-web",
		ProjectID: 62, // sitecore plus
		Old:       `"fa-icon" extension:html`,
		Crnt:      `"<crnt-icon" -"crnt-icon-button" extension:html`,
	},
	// This demonstrates the limitations; given they have the same name and
	// mostly the same params, there's no way to discern between theme and crnt
	// icons. Doing a loose query for the import has a lot of false positives,
	// but an exact query won't work due to there rarely being a single import
	// from that library.
	// Queries for the app will also be tricky as everything is wrapped in their custom components.
	{
		Name:      "icon-apps",
		ProjectID: 3202, // apps
		Old:       `"import { Icon } from \"@essent/themes\"" extension:tsx`,
		Crnt:      `"import { Icon } from \"@essent/crnt-react-native\"" extension:tsx`,
	},
}

// custom logger to allow for custom date/time format
// see https://stackoverflow.com/questions/26152993/go-logger-to-print-timestamp
type writer struct {
	io.Writer
	timeFormat string
}

func (w writer) Write(b []byte) (n int, err error) {
	return w.Writer.Write(append([]byte(time.Now().Format(w.timeFormat)), b...))
}

type gitlabLogger struct {
	log *log.Logger
}

func (c *gitlabLogger) Printf(format string, v ...interface{}) {
	c.log.Printf(format, v...)
}

// We could fetch a list of projects from gitlab but lazy.
var projectNames = map[int]string{
	62:   "Sitecore plus / Frontend",
	3202: "mobile-apps / Mobile Monorepo",
}

func updateData(db *sql.DB) {
	privateToken, exists := os.LookupEnv("PRIVATE_TOKEN")
	if !exists {
		log.Fatal("GitLab access token not set in environment variable PRIVATE_TOKEN")
	}

	// create and configure Gitlab API client
	options := []gitlab.ClientOptionFunc{}
	options = append(options, gitlab.WithBaseURL("https://gitlab.essent.nl/api/v4"))
	options = append(options, gitlab.WithCustomLogger(&gitlabLogger{log: log.New(&writer{os.Stdout, time.RFC3339Nano}, " [gitlab] ", 0)}))

	client, err := gitlab.NewClient(privateToken, options...)

	if err != nil {
		log.Fatalf("Failed to create gitlab client: %v", err)
	}

	search := &glclient.Search{
		Client:  client,
		Verbose: true,
	}

	// use one timestamp for all results
	now := time.Now()

	worker := func(queryPairsChan <-chan domain.QueryPair, results chan<- domain.ResultRow, wg *sync.WaitGroup) {
		defer wg.Done()
		for qp := range queryPairsChan {
			log.Printf("Running query %s...", qp.Name)
			oldResults, err1 := search.CountCodeByProject(qp.Old, qp.ProjectID)
			crntResults, err2 := search.CountCodeByProject(qp.Crnt, qp.ProjectID)

			if err := cmp.Or(err1, err2); err != nil {
				log.Fatalf("error querying code %v", err)
			}

			res := domain.ResultRow{
				Timestamp:   now,
				ProjectID:   qp.ProjectID,
				QueryName:   qp.Name,
				OldResults:  oldResults,
				CrntResults: crntResults,
			}

			results <- res
		}
	}

	tasks := make(chan domain.QueryPair, 5)
	results := make(chan domain.ResultRow, 5)
	var wg sync.WaitGroup

	// configure how many workers and thus simultaneous queries can run; while
	// more workers is more faster, it also puts a higher peak load on the
	// server.
	// 1 worker:  0,25s user 0,39s system 35% cpu 1,811 total
	// 3 workers: 0,26s user 0,40s system 55% cpu 1,199 total
	// 5 workers: 0,25s user 0,38s system 73% cpu 0,871 total
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go worker(tasks, results, &wg)
	}

	go func() {
		for _, qp := range queryPairs {
			tasks <- qp
		}
		close(tasks)
	}()

	// Fan-in: Close results channel once all workers complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Read the results from the result channel and accumulate them.
	// TODO: would it be "better" to pass the channel to the sqlite function and
	// have it read it? Something to ponder / measure. Note that this implementation
	// iterates over all values in the results channel, then the sqlite package
	// iterates over it again to do inserts one at a time. Also look into bulk
	// inserts; may need a query generator for that.
	// Go thingy; normally when converting one slice to another you know the
	// length, but since we're converting a channel to a slice we don't have
	// that information while ranging over it, so we can't use the index. We do
	// know what the length of the results will be though. However, if we pass
	// len(queryPairs) as the 2nd argument to make(), then append to it, we'd
	// get a slice that is len(queryPairs) of `nil` + len(queryPairs) of
	// results. Using the three arg version of make omits that, it sets the
	// length to 0 but the capacity to len(queryPairs).
	resultRows := make([]domain.ResultRow, 0, len(queryPairs))

	for res := range results {
		resultRows = append(resultRows, res)
	}

	if !dontPersist {
		if err := sqlite.SaveResults(db, resultRows); err != nil {
			log.Fatalf("error saving results: %v", err)
		}
	}

	writeTable(fmt.Sprintf("Queried results at %s", now), resultRows)
}

func writeTable(title string, results []domain.ResultRow) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.SetTitle(title)
	t.AppendHeader(table.Row{"Timestamp", "Project", "Query", "Old count", "CRNT count"})

	for _, row := range results {
		projectName, exists := projectNames[row.ProjectID]
		if !exists {
			projectName = strconv.Itoa(row.ProjectID)
		}
		t.AppendRow(table.Row{row.Timestamp.Format("2006-01-02 15:04:05"), projectName, row.QueryName, row.OldResults, row.CrntResults})
	}
	t.Render()
}
