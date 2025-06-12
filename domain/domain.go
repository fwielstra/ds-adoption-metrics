package domain

import "fmt"

type Project struct {
	ID   int
	Name string
	URL  string
}

type SearchQuery struct {
	Name      string
	Query     string
	Extension string
	ProjectID int // limit search to a single project
}

func (s SearchQuery) String() string {
	return fmt.Sprintf("'%s' query: '%s' extension: '%s' projectId: %d", s.Name, s.Query, s.Extension, s.ProjectID)
}

type SearchResult struct {
	ProjectID int
	// ?? what else do we really need from a search result?
}

// -func (q SearchQuery) toURLParam() (string, error) {
// -       if q.search == "" {
// -               return "", fmt.Errorf("no search query passed")
// -       }
// -
// -       result := q.search
// -
// -       if q.extension != "" {
// -               result = fmt.Sprintf("%s extension:%s", result, q.extension)
// -       }
// -
// -       return result, nil
// -}
