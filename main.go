package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"

	"database/sql"

	"github.com/jedib0t/go-pretty/v6/table"

	domain "github.com/fwielstra/crntmetrics/domain"
	glclient "github.com/fwielstra/crntmetrics/glclient"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	_ "modernc.org/sqlite"
)

const databasePath = "./data/adoption.db"

var reset bool
var verbose bool

type gitlabClient interface {
	ListProjects(client *gitlab.Client)
	SearchCode(client *gitlab.Client, query domain.SearchQuery) (domain.SearchResult, error)
}

type gitlabLogger struct {
}

func (c *gitlabLogger) Printf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

func main() {
	// setup flags
	flag.BoolVar(&reset, "reset", false, "Drops tables and resets all data")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose logging, including API calls")
	flag.Parse()

	// ensure database exists, trigger data reset if not.
	if _, err := os.Stat(databasePath); err != nil {
		reset = true
	}

	// read config
	privateToken, exists := os.LookupEnv("PRIVATE_TOKEN")
	if !exists {
		log.Fatal("GitLab access token not set in environment variable PRIVATE_TOKEN")
	}

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
	err = migrateTables(db)
	if err != nil {
		log.Fatalf("error creating tables: %v", err)
	}

	options := []gitlab.ClientOptionFunc{}
	options = append(options, gitlab.WithBaseURL("https://gitlab.essent.nl/api/v4"))

	if verbose {
		options = append(options, gitlab.WithCustomLogger(&gitlabLogger{}))
	}

	client, err := gitlab.NewClient(privateToken, options...)

	if err != nil {
		log.Fatalf("Failed to create gitlab client: %v", err)
	}

	// if reset {
	// 	// TODO: change so it only fetches the projects found in search results.
	// 	log.Println("fetching projects and loading into database")
	// 	count, err := glclient.LoadProjects(db, client)
	// 	if err != nil {
	// 		log.Fatalf("error loading projects: %v", err)
	// 	}
	// 	log.Printf("loaded %d projects", count)
	// } else {
	// 	// TODO: extract to database layer
	// 	rows, err := db.Query("select count(*) from projects;")
	// 	var count int
	// 	if err != nil {
	// 		log.Fatalf("error counting projects: %v", err)
	// 	}

	// 	for rows.Next() {
	// 		if err := rows.Scan(&count); err != nil {
	// 			log.Fatal(err)
	// 		}
	// 	}
	// 	log.Printf("There are %d projects in the database", count)
	// }

	type ProjectID = int

	queries := []domain.SearchQuery{
		{
			Name:      "bootstrap-primary-button",
			Query:     `class=\"btn btn-primary`,
			Extension: "html",
			ProjectID: 62, // sitecore plus
		},
		{
			Name:      "crnt-primary-button",
			Query:     `variant\"primary\"`,
			Extension: "html",
			ProjectID: 62,
		},
	}

	// TODO: fan out concurrency and / or backoff / rate limiting.
	// Rate limiting is also supported in the gitlab client; consider making it configurable / a CLI argument and enable it at night (when there will be few code changes)
	for _, q := range queries {

		results, err := glclient.SearchCode(client, q)
		if err != nil {
			log.Fatalf("error querying code %v", err)
		}
		total := len(results)
		grouped := make(map[ProjectID]int)
		for _, res := range results {
			grouped[res.ProjectID] += 1
		}

		log.Printf("results for query %s:", q)

		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.AppendSeparator()
		t.AppendHeader(table.Row{"PID", "Amount"})

		for k, v := range grouped {
			t.AppendRow(table.Row{k, v})
		}
		t.AppendSeparator()
		t.AppendRow(table.Row{"Query name", q.Name})
		t.AppendRow(table.Row{"Query:", q.Query})

		t.AppendSeparator()
		t.AppendFooter(table.Row{"Total", total})

		// t.SetStyle(table.StyleColoredBright)
		t.Render()
	}
}

func PrintJSON(obj interface{}) {
	bytes, _ := json.MarshalIndent(obj, "\t", "\t")
	fmt.Println(string(bytes))
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

const createQuery = `
CREATE TABLE IF NOT EXISTS projects (
	 id int primary key
	,name varchar
	,nameWithNamespace varchar
	,url varchar
)
`

// TODO: add db versioning to check if migration is needed.
func migrateTables(db *sql.DB) error {
	log.Println("Migrating tables...")

	if _, err := db.Exec(createQuery); err != nil {
		return err
	}

	log.Print("Tables created")

	return nil
}

func loadProjects(db *sql.DB, client *gitlab.Client) (int, error) {
	// get list of project IDs and names
	opts := &gitlab.ListProjectsOptions{}
	it, hasErr := gitlab.Scan(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
		return client.Projects.ListProjects(opts, p)
	})

	projects := slices.Collect(it)
	if err := hasErr(); err != nil {
		log.Printf("error fetching projects: %v", err)
	}

	log.Printf("fetched %d projects from API\n", len(projects))

	tx, err := db.Begin()
	if err != nil {
		return -1, fmt.Errorf("loadProjects(): error starting db transaction: %w", err)
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	// TODO: move preparation to init function (lookup if this is good practice)
	stmt, err := db.Prepare("INSERT INTO projects (id, name, nameWithNamespace, url) VALUES (?, ?, ?, ?);")
	if err != nil {
		return -1, fmt.Errorf("loadProjects(): error preparing statement: %v", err)
	}

	defer stmt.Close()

	for _, p := range projects {
		log.Printf("inserting project %d %s", p.ID, p.NameWithNamespace)
		if _, err := stmt.Exec(p.ID, p.Name, p.NameWithNamespace, p.WebURL); err != nil {
			return -1, fmt.Errorf("error inserting project %d %s: %v", p.ID, p.NameWithNamespace, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return -1, err
	}

	return len(projects), nil
}
