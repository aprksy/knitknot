package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var globalFlags struct {
	subgraph string
	file     string
}

var RootCmd = &cobra.Command{
	Use:   "knitknot",
	Short: "KnitKnot - A flexible property graph engine",
	Long: `KnitKnot: where data gets knitted into knots.

A lightweight, embeddable graph database with fluent querying and visualization.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Global setup if needed
	},
}

var verbose bool

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Register global flags
	RootCmd.PersistentFlags().StringVarP(
		&globalFlags.file,
		"file", "f",
		"",
		"Graph data file to load and save (e.g., data.gob)",
	)
}

func initConfig() {
	// Optional: config file logic later
}
