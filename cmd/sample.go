package cmd

import (
	"fmt"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/types"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
	"github.com/spf13/cobra"
)

var sampleCmd = &cobra.Command{
	Use:   "generate-sample",
	Short: "Generate sample data",
	Long:  "GEnerate sample data for you to get started with KnitKnot.",
	RunE:  runGenerateSample,
}

func init() {
	RootCmd.AddCommand(sampleCmd)
}

func runGenerateSample(cmd *cobra.Command, args []string) error {
	storage := inmem.New()
	engine := graph.NewGraphEngine(storage)

	seedSampleData(engine)

	verbs := map[string]types.Verb{
		"make_purchase_in":   {TargetLabel: "channel", MatchOn: "name"},
		"purchase":           {TargetLabel: "commodity", MatchOn: "name"},
		"make_payment_using": {TargetLabel: "payment_method", MatchOn: "name"},
	}

	for verb, dest := range verbs {
		engine.RegisterVerb(verb, dest)
	}

	fmt.Printf("Sample graph is generated with the following details:\n")
	err := execListVerbs(engine, cmd.OutOrStdout())
	if err != nil {
		return err
	}
	fmt.Println()

	err = execSave(engine, globalFlags.file, cmd.OutOrStdout())
	if err != nil {
		return err
	}
	return nil
}
