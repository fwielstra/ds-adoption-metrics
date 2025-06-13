package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"

	"database/sql"

	cmd "github.com/fwielstra/crntmetrics/cmd"
	sqlite "github.com/fwielstra/crntmetrics/sqlite"

	"github.com/go-echarts/go-echarts/v2/opts"

	_ "modernc.org/sqlite"
)

const databasePath = "./data/adoption.db"

func main() {
	reset := false
	// ensure database exists, trigger data reset if not.
	if _, err := os.Stat(databasePath); err != nil {
		reset = true
	}

	// create and migrate database
	if reset {
		log.Printf("dropping database %s", databasePath)
		if err := os.Remove(databasePath); err != nil {
			// error can be 'file does not exist' which is fine.
			log.Println(err)
		}
	}

	db, err := createDatabase()

	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	if reset {
		if err := sqlite.MigrateTables(db); err != nil {
			log.Fatalf("error creating tables: %v", err)
		}
	}

	// run command
	cmd.Execute(db)
}

func createDatabase() (*sql.DB, error) {
	// ensure data folder exists
	_ = os.MkdirAll("./data", 0755)
	db, err := sql.Open("sqlite", databasePath)

	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	return db, nil
}

// generate random data for bar chart
func generateBarItems() []opts.BarData {
	items := make([]opts.BarData, 0)
	for i := 0; i < 7; i++ {
		items = append(items, opts.BarData{Value: rand.Intn(300)})
	}
	return items
}
