package glclient

import (
	"log"
	"slices"

	domain "github.com/fwielstra/crntmetrics/domain"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Search struct {
	Client  *gitlab.Client
	Verbose bool
}

// TODO: if we only want the count we don't need to query all pages if we already limit to 1 project. But we'll want to search multiple projects eventually.
func (s *Search) SearchCodeByProject(query string, projectID int) ([]*domain.SearchResult, error) {
	opts := &gitlab.SearchOptions{}

	it, hasErr := gitlab.Scan(func(p gitlab.PaginationOptionFunc) ([]*domain.SearchResult, *gitlab.Response, error) {
		blobs, resp, err := s.Client.Search.BlobsByProject(projectID, query, opts, p)
		if err != nil {
			return nil, nil, err
		}

		if s.Verbose {
			log.Printf("fetched page %d of %d for query '%s'", resp.CurrentPage, resp.TotalPages, query)
		}

		results := make([]*domain.SearchResult, len(blobs))
		for i, blob := range blobs {
			results[i] = &domain.SearchResult{
				ProjectID: blob.ProjectID,
			}
		}
		return results, resp, err
	})

	allResults := slices.Collect(it)
	if err := hasErr(); err != nil {
		log.Printf("error fetching projects: %v", err)
		return nil, err
	}

	return allResults, nil
}
