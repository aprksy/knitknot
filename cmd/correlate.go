package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/extensions/dfir"
	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var correlateCmd = &cobra.Command{
	Use:   "correlate",
	Short: "Correlate case observables against the CTI graph",
	Long:  "Match DFIR case observables (CSV) against indicator patterns in the graph and report reachable adversary context.",
	RunE:  runCorrelate,
}

var correlateFlags struct {
	observables string
	format      string
	workspace   string
	source      string
}

func init() {
	correlateCmd.Flags().StringVarP(&correlateFlags.observables, "observables", "O", "", "Case observables CSV file (type,value[,context])")
	correlateCmd.Flags().StringVar(&correlateFlags.format, "format", "text", "Output format (text, json)")
	correlateCmd.Flags().StringVar(&correlateFlags.workspace, "workspace", "", "Write a projected case workspace graph to this .gob file")
	correlateCmd.Flags().StringVar(&correlateFlags.source, "source", "case", "source_feed recorded on case observable nodes")
	RootCmd.AddCommand(correlateCmd)
}

func runCorrelate(cmd *cobra.Command, args []string) error {
	if correlateFlags.observables == "" {
		return fmt.Errorf("usage: knitknot correlate --observables <case.csv> -f <graph.gob>: missing --observables")
	}

	f, err := os.Open(correlateFlags.observables)
	if err != nil {
		return fmt.Errorf("open %s: %w", correlateFlags.observables, err)
	}
	defer f.Close()

	caseObs, err := dfir.ParseCaseCSV(f)
	if err != nil {
		return err
	}

	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}
	if engine, err = resolveStoreBackend(engine); err != nil {
		return err
	}

	matches := dfir.Correlate(engine.Storage(), caseObs)

	if correlateFlags.workspace != "" {
		wsEngine := graph.NewGraphEngine(inmem.New())
		if err := wireExtensionVerbs(wsEngine); err != nil {
			return err
		}
		stats, err := dfir.BuildWorkspace(engine.Storage(), wsEngine.Storage(), caseObs, correlateFlags.source)
		if err != nil {
			return err
		}
		if err := SaveGraph(wsEngine, correlateFlags.workspace); err != nil {
			return err
		}
		fmt.Printf("Workspace: %d observables, %d indicators, %d malware, %d campaigns, %d actors, %d ttps, %d edges\n",
			stats.Observables, stats.Indicators, stats.Malware, stats.Campaigns, stats.Actors, stats.TTPs, stats.Edges)
	}

	switch correlateFlags.format {
	case "text":
		for _, m := range matches {
			if m.Context != "" {
				fmt.Printf("MATCH %s %q (%s)\n", m.Observable.Type, m.Observable.Value, m.Context)
			} else {
				fmt.Printf("MATCH %s %q\n", m.Observable.Type, m.Observable.Value)
			}
			for _, r := range m.Indicators {
				fmt.Printf("  indicator: %s\n", r.Name)
			}
			for _, r := range m.Malware {
				fmt.Printf("  malware:   %s\n", r.Name)
			}
			for _, r := range m.Campaigns {
				fmt.Printf("  campaign:  %s\n", r.Name)
			}
			for _, r := range m.Actors {
				fmt.Printf("  actor:     %s\n", r.Name)
			}
			for _, r := range m.TTPs {
				fmt.Printf("  ttp:       %s\n", r.Name)
			}
		}
		fmt.Printf("%d of %d case observables matched\n", len(matches), len(caseObs))
		return nil
	case "json":
		data, err := json.Marshal(matches)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format %q: want text or json", correlateFlags.format)
	}
}
