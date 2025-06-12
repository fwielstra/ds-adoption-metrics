package glclient

import (
	"fmt"
	"log"
	"slices"

	domain "github.com/fwielstra/crntmetrics/domain"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func SearchCodeByProject(client *gitlab.Client, query domain.SearchQuery, projectID int) ([]*domain.SearchResult, error) {
	opts := &gitlab.SearchOptions{}

	queryString := query.Query

	if query.Extension != "" {
		queryString = fmt.Sprintf("%s extension:%s", queryString, query.Extension)
	}

	it, hasErr := gitlab.Scan(func(p gitlab.PaginationOptionFunc) ([]*domain.SearchResult, *gitlab.Response, error) {
		blobs, resp, err := client.Search.BlobsByProject(projectID, queryString, opts, p)
		if err != nil {
			return nil, nil, err
		}
		log.Printf("fetched page %d of %d", resp.CurrentPage, resp.TotalPages)

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
