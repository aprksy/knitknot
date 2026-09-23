package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
)

var globalFlags struct {
	subgraph string
	file     string
}

// store: resolver — --store flag backing (separate from globalFlags so the
// existing -f plumbing and its tests stay untouched).
var storeFlag string

var RootCmd = &cobra.Command{
	Use:   "knitknot",
	Short: "KnitKnot - A flexible property graph engine",
	Long: `KnitKnot: where data gets knitted into knots.

A lightweight, embeddable graph database with fluent querying and visualization.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Global setup if needed
	},
}

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
	RootCmd.PersistentFlags().StringVar(
		&storeFlag,
		"store",
		"",
		"Store URI selecting the storage backend (e.g., gob:data.gob)",
	)

	ensureDefaultStore()
}

// store: backend — default gob registration, idempotent under re-entry.
var storeOnce sync.Once

func ensureDefaultStore() {
	storeOnce.Do(func() { portstorage.Register(gobBackend{}) })
}

func initConfig() {
	// Optional: config file logic later
}
