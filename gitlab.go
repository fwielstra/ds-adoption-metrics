package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
)

type GitlabClient struct {
	privateToken string
}

func (c *GitlabClient) Request(method string, path string, body io.Reader) (*http.Response, error) {
	log.Printf("%s %s", method, path)

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

// runs the given API request for all pages returned by the value of the "x-pages" response header of the first result.
// Returns a []byte for every response. Kinda memory heavy for bigger responses though, ideally it would stream the results.
func (c *GitlabClient) GetAll(path string) ([][]byte, error) {
	resp, err := c.Get(path)

	if err != nil {
		return nil, fmt.Errorf("GetAll(): error querying gitlab: %w", err)
	}

	pages, err := strconv.Atoi(resp.Header.Get("x-total-pages"))
	if err != nil {
		return nil, fmt.Errorf("GetAll(): error reading x-total-pages, are there more than 10.000 results? %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var result [][]byte
	result = append(result, body)

	// should be skipped if there's only one page
	for page := 2; page <= pages; page++ {

		parsedPath, err := url.Parse(path)

		if err != nil {
			return nil, fmt.Errorf("SearchAll(): failed to parse URL %s: %w", path, err)
		}

		values := parsedPath.Query()
		values.Add("page", strconv.Itoa(page))
		parsedPath.RawQuery = values.Encode()

		pagedResults, err := c.Get(parsedPath.String())

		if err != nil {
			log.Printf("SearchAll(): error fetching page %d of %d: %v, skipping...", page, pages, err)
			continue
		}

		pagedBody, err := io.ReadAll(pagedResults.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %w", err)
		}

		result = append(result, pagedBody)
	}

	return result, nil
}

type SearchQuery struct {
	// unique name that identifies the query
	name string

	// the search query, see https://docs.gitlab.com/user/search/advanced_search/ and https://www.elastic.co/docs/reference/query-languages/query-dsl/query-dsl-simple-query-string-query
	// Make sure to escape quotes using \"; you may need to use triple escaping.
	search string

	// optional, search only in specific extensions like "html"
	extension string
}

func (q SearchQuery) toURLParam() (string, error) {
	if q.search == "" {
		return "", fmt.Errorf("no search query passed")
	}

	result := q.search

	if q.extension != "" {
		result = fmt.Sprintf("%s extension:%s", result, q.extension)
	}

	return result, nil
}

// run a single search query, returning the headers which contain paging information
// see https://docs.gitlab.com/api/rest/#pagination
func (c *GitlabClient) Search(queryUrl string) ([]BlobSearchResult, http.Header, error) {
	// first search query
	resp, err := c.Get(queryUrl)

	if err != nil {
		return nil, nil, fmt.Errorf("error searching gitlab: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("error reading response: %w", err)
	}

	var results []BlobSearchResult
	err = json.Unmarshal(body, &results)

	if err != nil {
		return nil, nil, fmt.Errorf("error parsing response: %w", err)
	}

	return results, resp.Header, nil
}

func (c *GitlabClient) SearchAll(query SearchQuery) ([]BlobSearchResult, error) {
	param, err := query.toURLParam()

	if err != nil {
		return nil, fmt.Errorf("Search(): error generating search query %w", err)
	}

	queryUrl := "https://gitlab.essent.nl/api/v4/search?scope=blobs&search=" + url.QueryEscape(param)
	queryResults, err := c.GetAll(queryUrl)

	if err != nil {
		return nil, fmt.Errorf("SearchAll(): error querying gitlab %w", err)
	}

	// parse output to BlobSearchResult
	var allResults []BlobSearchResult

	for _, page := range queryResults {
		var result []BlobSearchResult
		err = json.Unmarshal(page, &result)
		if err != nil {
			return nil, fmt.Errorf("error unmarshaling JSON blob: %w", err)
		}
		allResults = append(allResults, result...)
	}

	return allResults, nil
}

func (c *GitlabClient) GetProjects() ([]Project, error) {
	pages, err := c.GetAll("https://gitlab.essent.nl/api/v4/projects")

	if err != nil {
		return nil, fmt.Errorf("GetProjects(): error fetching projects from gitlab: %w", err)
	}

	var projects []Project

	for _, page := range pages {
		var result []Project
		err = json.Unmarshal(page, &result)
		if err != nil {
			fmt.Printf("%s", string(page))
			return nil, fmt.Errorf("GetProjects(): error unmarshaling JSON blob: %w", err)
		}
		projects = append(projects, result...)
	}

	return projects, nil
}

type Project struct {
	Id          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func Prettify(data []byte) (string, error) {
	var prettyJSON bytes.Buffer

	err := json.Indent(&prettyJSON, data, "", "\t")

	if err != nil {
		return "", fmt.Errorf("error prettifying response: %w", err)
	}
	return prettyJSON.String(), nil
}

type BlobSearchResult struct {
	BaseName  string `json:"basename"`
	Data      string `json:"data"`
	Path      string `json:"path"`
	FileName  string `json:"filename"`
	Id        string `json:"id"`
	Ref       string `json:"ref"`
	StartLine int    `json:"startline"`
	ProjectId int    `json:"project_id"`
}
