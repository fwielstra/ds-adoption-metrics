package glclient

import (
	"database/sql"
	"fmt"
	"log"
	"slices"

	domain "github.com/fwielstra/crntmetrics/domain"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func ListProjects(client *gitlab.Client) ([]*domain.Project, error) {
	opts := &gitlab.ListProjectsOptions{}

	// fetches all pages and immediately reduces them to our domain project object, the gitlab object is pretty heavyweight.
	it, hasErr := gitlab.Scan(func(p gitlab.PaginationOptionFunc) ([]*domain.Project, *gitlab.Response, error) {
		projects, response, err := client.Projects.ListProjects(opts, p)

		if err != nil {
			return nil, response, fmt.Errorf("glclient.ListProjects(): error fetching projects page: %w", err)
		}

		result := make([]*domain.Project, len(projects))
		for i, project := range projects {
			result[i] = &domain.Project{
				ID:   project.ID,
				Name: project.NameWithNamespace,
				URL:  project.WebURL,
			}
		}

		return result, response, err
	})

	if err := hasErr(); err != nil {
		return nil, fmt.Errorf("glclient.ListProjects(): error fetching projects: %w", err)
	}

	return slices.Collect(it), nil
}

func LoadProjects(db *sql.DB, client *gitlab.Client) (int, error) {
	// get list of project IDs and names
	opts := &gitlab.ListProjectsOptions{}
	it, hasErr := gitlab.Scan(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Project, *gitlab.Response, error) {
		return client.Projects.ListProjects(opts, p)
	})

	projects := slices.Collect(it)
	if err := hasErr(); err != nil {
		log.Printf("error fetching projects: %v", err)
	}

	log.Printf("fetched %d projects from API\n", len(projects))

	tx, err := db.Begin()
	if err != nil {
		return -1, fmt.Errorf("loadProjects(): error starting db transaction: %w", err)
	}
	defer tx.Rollback() // The rollback will be ignored if the tx has been committed later in the function.

	// TODO: move preparation to init function (lookup if this is good practice)
	stmt, err := db.Prepare("INSERT INTO projects (id, name, nameWithNamespace, url) VALUES (?, ?, ?, ?);")
	if err != nil {
		return -1, fmt.Errorf("loadProjects(): error preparing statement: %v", err)
	}

	defer stmt.Close()

	for _, p := range projects {
		log.Printf("inserting project %d %s", p.ID, p.NameWithNamespace)
		if _, err := stmt.Exec(p.ID, p.Name, p.NameWithNamespace, p.WebURL); err != nil {
			return -1, fmt.Errorf("error inserting project %d %s: %v", p.ID, p.NameWithNamespace, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return -1, err
	}

	return len(projects), nil
}
