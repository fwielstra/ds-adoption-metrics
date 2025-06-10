package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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

	resp, err := client.Search()
	if err != nil {
		log.Print(err)
	}

	fmt.Print(resp)
}

func createDatabase() (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:adoption.db?cache=shared&mode=rw")

	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	return db, nil
}

type GitlabClient struct {
	privateToken string
}

func (c *GitlabClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, path, body)
	if err != nil {
		return nil, fmt.Errorf("error querying Gitlab: %w", err)
	}
	req.Header.Add("PRIVATE-TOKEN", c.privateToken)
	return client.Do(req)
}

func (c *GitlabClient) Get(path string) (*http.Response, error) {
	return c.Request("GET", path, nil)
}

func (c *GitlabClient) Search() (string, error) {
	search := url.QueryEscape("crnt-button extension:html")
	resp, err := c.Get("https://gitlab.essent.nl/api/v4/search?scope=blobs&search=" + search)

	if err != nil {
		return "", fmt.Errorf("error searching gitlab: %w", err)
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return "", fmt.Errorf("error parsing response: %w", err)
	}

	return string(body), nil
}
