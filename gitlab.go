package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

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

type SearchQuery struct {
	search    string
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

func (c *GitlabClient) Search(query SearchQuery) ([]BlobSearchResult, error) {
	param, err := query.toURLParam()

	if err != nil {
		return nil, fmt.Errorf("Search(): error generating search query %w", err)
	}

	resp, err := c.Get("https://gitlab.essent.nl/api/v4/search?scope=blobs&search=" + url.QueryEscape(param))

	if err != nil {
		return nil, fmt.Errorf("error searching gitlab: %w", err)
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	var results []BlobSearchResult
	err = json.Unmarshal(body, &results)

	if err != nil {
		return nil, fmt.Errorf("error parsing response: %w", err)
	}

	return results, nil
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
