package sqlite

import (
	"database/sql"
	"log"
)

const createResultsTable = `
CREATE TABLE IF NOT EXISTS results (
	timestamp DATETIME NOT NULL,
	projectId INTEGER,
	query TEXT,
	oldResults INTEGER,
	crntResults INTEGER
);
`

// TODO: add db versioning to check if migration is needed.
func MigrateTables(db *sql.DB) error {
	log.Println("Migrating tables...")

	if _, err := db.Exec(createResultsTable); err != nil {
		return err
	}

	log.Print("Tables created")

	return nil
}
