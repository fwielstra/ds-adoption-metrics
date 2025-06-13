package cmd

import (
	"database/sql"
	"os"

	"github.com/spf13/cobra"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(db *sql.DB) {

	// rootCmd represents the base command when called without any subcommands
	var rootCmd = &cobra.Command{
		Use:   "crntmetrics",
		Short: "A Gitlab code frequency fetcher and reporter",
		Long: `This application runs a number of code search queries on a Gitlab
instance in pairs, to track usage of code over time. Useful to generate
analytics and reports on e.g. the adoption of the CRNT Design System.
`,
		// Uncomment the following line if your bare application
		// has an action associated with it:
		// Run: func(cmd *cobra.Command, args []string) { },
	}

	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	rootCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging, including API calls")

	rootCmd.AddCommand(NewUpdateCmd(db))
	rootCmd.AddCommand(NewGenerateChartCmd(db))
	rootCmd.AddCommand(serveCmd)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobrainit.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.

}
