package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"database/sql"

	_ "modernc.org/sqlite"
)

func main() {
	fmt.Println("hello world")

	// read config
	privateToken, exists := os.LookupEnv("PRIVATE_TOKEN")
	if !exists {
		log.Fatal("GitLab access token not set in environment variable PRIVATE_TOKEN")
	}

	db, err := createDatabase()

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()
	fmt.Printf("Database opened successfully: %v\n", db)

	client := &GitlabClient{
		privateToken: privateToken,
	}

	// get list of project IDs and names
	projects, err := client.GetProjects()
	if err != nil {
		log.Printf("error fetching projects: %w", err)
	}

	PrintJSON(projects)

	// TODO: create table, load projects; do not reload projects unless explicitly told to.

	// queries := []SearchQuery{
	// 	{
	// 		name:      "bootstrap-primary-button",
	// 		search:    `class=\"btn btn-primary`,
	// 		extension: "html",
	// 	},
	// }

	// for _, q := range queries {
	// 	resp, err := client.SearchAll(q)

	// 	if err != nil {
	// 		log.Print(err)
	// 	}

	// 	total := len(resp)
	// 	grouped := make(map[ProjectID]int)

	// 	for _, res := range resp {
	// 		// PrintJSON(res)
	// 		grouped[res.ProjectId] += 1
	// 	}

	// 	log.Printf("total %d, per group %#v", total, grouped)
	// }
}

type ProjectID = int

func PrintJSON(obj interface{}) {
	bytes, _ := json.MarshalIndent(obj, "\t", "\t")
	fmt.Println(string(bytes))
}

func createDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:adoption.db?cache=shared&mode=rw")

	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	return db, nil
}
