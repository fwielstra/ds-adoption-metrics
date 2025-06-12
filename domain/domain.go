package domain

import "time"

type Project struct {
	ID   int
	Name string
	URL  string
}

type QueryPair struct {
	Name      string
	ProjectID int
	Old       string
	Crnt      string
}

type SearchResult struct {
	ProjectID int
}

type ResultRow struct {
	Timestamp   time.Time
	ProjectID   int
	QueryName   string
	OldResults  int
	CrntResults int
}
