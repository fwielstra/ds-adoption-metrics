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

	crntButtonQuery := SearchQuery{
		search:    "crnt-button",
		extension: "html",
	}

	resp, err := client.Search(crntButtonQuery)
	if err != nil {
		log.Print(err)
	}

	for _, res := range resp {
		PrintJSON(res)
	}
}

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
