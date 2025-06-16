/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"cmp"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
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
		Name:      "primary-button",
		ProjectID: 62, // sitecore plus
		Old:       `class=\"btn btn-primary extension:html`,
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"primary\" extension:html`,
	},
	{
		Name:      "secondary-button",
		ProjectID: 62,                                           // sitecore plus
		Old:       `class=\""btn btn-secondary" extension:html`, // note that there is one use case where the button type is dynamic
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"secondary\" extension:html`,
	},
	{
		Name:      "tertiary-button",
		ProjectID: 62, // sitecore plus
		Old:       `class=\""btn btn-link" extension:html`,
		Crnt:      `("crnt-button" | "crnt-button-alt") variant=\"tertiary\" extension:html`,
	},
	{
		Name:      "fa-icon",
		ProjectID: 62, // sitecore plus
		Old:       `"fa-icon" extension:html`,
		Crnt:      `"<crnt-icon" -"crnt-icon-button" extension:html`,
	},
}

type gitlabLogger struct {
}

func (c *gitlabLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// We could fetch a list of projects from gitlab but lazy.
var projectNames = map[int]string{
	62: "Sitecore plus / Frontend",
}

func updateData(db *sql.DB) {
	privateToken, exists := os.LookupEnv("PRIVATE_TOKEN")
	if !exists {
		log.Fatal("GitLab access token not set in environment variable PRIVATE_TOKEN")
	}

	// create and configure Gitlab API client
	options := []gitlab.ClientOptionFunc{}
	options = append(options, gitlab.WithBaseURL("https://gitlab.essent.nl/api/v4"))
	options = append(options, gitlab.WithCustomLogger(&gitlabLogger{}))

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

	// TODO: fan out concurrency and / or backoff / rate limiting.
	results := make([]domain.ResultRow, len(queryPairs))
	for i, qp := range queryPairs {
		log.Printf("Running query %s...", qp.Name)
		oldResults, err1 := search.CountCodeByProject(qp.Old, qp.ProjectID)
		crntResults, err2 := search.CountCodeByProject(qp.Crnt, qp.ProjectID)

		if err := cmp.Or(err1, err2); err != nil {
			log.Fatalf("error querying code %v", err)
		}

		results[i] = domain.ResultRow{
			Timestamp:   now,
			ProjectID:   qp.ProjectID,
			QueryName:   qp.Name,
			OldResults:  oldResults,
			CrntResults: crntResults,
		}
	}

	if !dontPersist {
		if err := sqlite.SaveResults(db, results); err != nil {
			log.Fatalf("error saving results: %v", err)
		}
	}

	writeTable(fmt.Sprintf("Queried results at %s", now), results)
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
