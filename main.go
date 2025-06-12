package main

import (
	"cmp"
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

	queryPairs := []domain.QueryPair{
		{
			Name:      "primary-button",
			ProjectID: 62, // sitecore plus
			Old: domain.SearchQuery{
				Query:     `class=\"btn btn-primary`,
				Extension: "html",
			},
			Crnt: domain.SearchQuery{
				Query:     `variant\"primary\"`,
				Extension: "html",
			},
		},
	}

	type resultRow struct {
		projectId   int
		oldQuery    string
		crntQuery   string
		oldResults  int
		crntResults int
	}

	results := make([]resultRow, len(queryPairs))

	// TODO: fan out concurrency and / or backoff / rate limiting.
	// Rate limiting is also supported in the gitlab client; consider making it configurable / a CLI argument and enable it at night (when there will be few code changes)
	for i, qp := range queryPairs {

		oldResults, err1 := glclient.SearchCodeByProject(client, qp.Old, qp.ProjectID)
		crntResults, err2 := glclient.SearchCodeByProject(client, qp.Crnt, qp.ProjectID)

		if err := cmp.Or(err1, err2); err != nil {
			log.Fatalf("error querying code %v", err)
		}
		results[i] = resultRow{
			projectId:   qp.ProjectID,
			oldQuery:    qp.Old.String(),
			crntQuery:   qp.Crnt.String(),
			oldResults:  len(oldResults),
			crntResults: len(crntResults),
		}
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"PID", "Old query", "CRNT query", "Old count", "CRNT count"})

	for _, row := range results {
		t.AppendRow(table.Row{row.projectId, row.oldQuery, row.crntQuery, row.oldResults, row.crntResults})
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
