package sqlite

import (
	"database/sql"
	"log"
	"time"

	"github.com/fwielstra/crntmetrics/domain"
)

// both sql.DB and sql.Tx implement this
type Executor interface {
	Exec(query string, args ...any) (sql.Result, error)
}

func SaveResult(exe Executor, result domain.ResultRow) error {
	if _, err := exe.Exec("INSERT INTO results (timestamp, projectId, query, oldResults, crntResults) VALUES (?, ?, ?, ?, ?)", result.Timestamp.UnixMilli(), result.ProjectID, result.QueryName, result.OldResults, result.CrntResults); err != nil {
		return err
	}

	log.Printf("result for query %s inserted", result.QueryName)

	return nil
}

func SaveResults(db *sql.DB, results []domain.ResultRow) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	// we could do a bulk insert query but lazy / not performance critical / this is fast enough.
	for _, res := range results {
		if err := SaveResult(tx, res); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func LoadResults(db *sql.DB) ([]domain.ResultRow, error) {
	rows, err := db.Query("SELECT timestamp, projectId, query, oldResults, crntResults FROM results ORDER BY timestamp ASC;")
	if err != nil {
		return nil, err
	}

	var results []domain.ResultRow
	for rows.Next() {
		var res domain.ResultRow
		var ts int64
		if err := rows.Scan(&ts, &res.ProjectID, &res.QueryName, &res.OldResults, &res.CrntResults); err != nil {
			return nil, err
		}
		res.Timestamp = time.UnixMilli(ts)
		results = append(results, res)
	}

	return results, nil
}
