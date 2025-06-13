/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/fwielstra/crntmetrics/domain"
	"github.com/fwielstra/crntmetrics/sqlite"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/spf13/cobra"
)

func NewGenerateChartCmd(db *sql.DB) *cobra.Command {
	return &cobra.Command{
		Use:   "generateChart",
		Short: "Generates a chart for all results or the specified command",
		Long:  ``,
		Run: func(cmd *cobra.Command, args []string) {
			query := "all"
			if len(args) > 0 {
				query = args[0]
			}

			var results []domain.ResultRow
			var err error

			if query == "all" {
				results, err = sqlite.LoadResults(db)
			} else {
				results, err = sqlite.LoadQueryResults(db, query)
			}

			if err != nil {
				log.Fatal(err)
			}

			writeChart(fmt.Sprintf("CRNT Adoption Rate for %s", query), query, results)
		},
	}
}

func writeChart(title string, filename string, results []domain.ResultRow) {
	bar := charts.NewLine()
	bar.SetGlobalOptions(charts.WithTitleOpts(opts.Title{
		Title: title,
	}))

	dates := make([]string, len(results))
	old := make([]opts.LineData, len(results))
	crnt := make([]opts.LineData, len(results))
	for i, res := range results {
		dates[i] = res.Timestamp.Format("2006-01-02 15:04:05")
		old[i] = opts.LineData{Value: res.OldResults}
		crnt[i] = opts.LineData{Value: res.CrntResults}
	}

	bar.SetXAxis(dates).
		AddSeries("Old", old).
		AddSeries("CRNT", crnt)

	f, _ := os.Create(fmt.Sprintf("%s.html", filename))
	bar.Render(f)

	log.Printf("generated chart at %s", f.Name())
}
