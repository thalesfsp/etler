// Package cmd provides the root command for the etler CLI.
package main

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "etler",
	Short: "ETLer is a powerful tool for extracting, transforming, and loading data.",
	Long: `ETLer is is a powerful tool for extracting, transforming,
and loading data from various sources and destinations (Adapters).

The project is designed to be flexible, allowing developers
to write custom pipeline stages. The pipeline also supports
concurrent processing of data, ensuring efficient and speedy
data processing.

Overall, ETLer is a valuable asset for any organization looking
to streamline and optimize its data management and analysis.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.etler.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
