package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"database/sql"

	"github.com/jedib0t/go-pretty/v6/table"

	domain "github.com/fwielstra/crntmetrics/domain"
	glclient "github.com/fwielstra/crntmetrics/glclient"
	sqlite "github.com/fwielstra/crntmetrics/sqlite"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	_ "modernc.org/sqlite"
)

const databasePath = "./data/adoption.db"

var reset bool
var verbose bool
var update bool

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
// TODO: put in database
var projectNames = map[int]string{
	62: "Sitecore plus / Frontend",
}

func main() {
	// setup flags
	flag.BoolVar(&reset, "reset", false, "Drops tables and resets all data")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging, including API calls")
	flag.BoolVar(&update, "update", false, "Run queries and add results to database")
	flag.Parse()

	// ensure database exists, trigger data reset if not.
	if _, err := os.Stat(databasePath); err != nil {
		reset = true
	}

	// create and migrate database
	if reset {
		log.Printf("dropping database %s", databasePath)
		if err := os.Remove(databasePath); err != nil {
			// error can be 'file does not exist' which is fine.
			log.Println(err)
		}
	}

	db, err := createDatabase()

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()
	log.Printf("Database opened successfully: %v\n", db)

	if reset {
		if err := sqlite.MigrateTables(db); err != nil {
			log.Fatalf("error creating tables: %v", err)
		}
	}

	// execute queries and add results to database if update flag was passed
	if update {
		updateData(db)
	}

	// read all results from database
	allResults, err := sqlite.LoadResults(db)
	if err != nil {
		log.Fatal(err)
	}

	writeTable("Historic data", allResults)
}

func updateData(db *sql.DB) {
	// read config
	privateToken, exists := os.LookupEnv("PRIVATE_TOKEN")
	if !exists {
		log.Fatal("GitLab access token not set in environment variable PRIVATE_TOKEN")
	}

	// create and configure Gitlab API client
	options := []gitlab.ClientOptionFunc{}
	options = append(options, gitlab.WithBaseURL("https://gitlab.essent.nl/api/v4"))

	if verbose {
		options = append(options, gitlab.WithCustomLogger(&gitlabLogger{}))
	}

	client, err := gitlab.NewClient(privateToken, options...)

	if err != nil {
		log.Fatalf("Failed to create gitlab client: %v", err)
	}

	search := &glclient.Search{
		Client:  client,
		Verbose: verbose,
	}
	// use one timestamp for all results
	now := time.Now()

	// TODO: fan out concurrency and / or backoff / rate limiting.
	// Rate limiting is also supported in the gitlab client; consider making it configurable / a CLI argument and enable it at night (when there will be few code changes)
	results := make([]domain.ResultRow, len(queryPairs))
	for i, qp := range queryPairs {

		oldResults, err1 := search.SearchCodeByProject(qp.Old, qp.ProjectID)
		crntResults, err2 := search.SearchCodeByProject(qp.Crnt, qp.ProjectID)

		if err := cmp.Or(err1, err2); err != nil {
			log.Fatalf("error querying code %v", err)
		}

		results[i] = domain.ResultRow{
			Timestamp:   now,
			ProjectID:   qp.ProjectID,
			QueryName:   qp.Name,
			OldResults:  len(oldResults),
			CrntResults: len(crntResults),
		}
	}

	if err := sqlite.SaveResults(db, results); err != nil {
		log.Fatalf("error saving results: %v", err)
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

func createDatabase() (*sql.DB, error) {
	// ensure data folder exists
	_ = os.MkdirAll("./data", 0755)
	db, err := sql.Open("sqlite", databasePath)

	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	return db, nil
}
