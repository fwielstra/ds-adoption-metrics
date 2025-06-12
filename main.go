package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strconv"

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
	SearchCode(client *gitlab.Client, query string) (domain.SearchResult, error)
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

	search := &glclient.Search{
		Client:  client,
		Verbose: verbose,
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

	type resultRow struct {
		projectId   int
		query       string
		oldResults  int
		crntResults int
	}

	results := make([]resultRow, len(queryPairs))

	// TODO: fan out concurrency and / or backoff / rate limiting.
	// Rate limiting is also supported in the gitlab client; consider making it configurable / a CLI argument and enable it at night (when there will be few code changes)
	for i, qp := range queryPairs {

		oldResults, err1 := search.SearchCodeByProject(qp.Old, qp.ProjectID)
		crntResults, err2 := search.SearchCodeByProject(qp.Crnt, qp.ProjectID)

		if err := cmp.Or(err1, err2); err != nil {
			log.Fatalf("error querying code %v", err)
		}

		results[i] = resultRow{
			projectId:   qp.ProjectID,
			query:       qp.Name,
			oldResults:  len(oldResults),
			crntResults: len(crntResults),
		}
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Project", "Query", "Old count", "CRNT count"})

	for _, row := range results {
		projectName, exists := projectNames[row.projectId]
		if !exists {
			projectName = strconv.Itoa(row.projectId)
		}
		t.AppendRow(table.Row{projectName, row.query, row.oldResults, row.crntResults})
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
