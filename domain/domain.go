package domain

import "fmt"

type Project struct {
	ID   int
	Name string
	URL  string
}

type SearchQuery struct {
	Query     string
	Extension string
}

func (s SearchQuery) String() string {
	return fmt.Sprintf("%s extension: '%s'", s.Query, s.Extension)
}

type QueryPair struct {
	Name      string
	ProjectID int
	Old       SearchQuery
	Crnt      SearchQuery
}

type SearchResult struct {
	ProjectID int
}
