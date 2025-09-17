package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  `Prints the version, git commit, and build date of KnitKnot.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("knitknot version %s\n", version)
		fmt.Printf("commit: %s\n", commit)
		fmt.Printf("built with: %s\n", runtime.Version())
		if buildDate != "unknown" {
			fmt.Printf("build date: %s\n", buildDate)
		}
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
