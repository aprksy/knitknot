package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/extensions/dfir"
	"github.com/aprksy/knitknot/pkg/graph"
	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

var correlateCmd = &cobra.Command{
	Use:   "correlate",
	Short: "Correlate case observables against the CTI graph",
	Long:  "Match DFIR case observables (CSV) against indicator patterns in the graph and report reachable adversary context.",
	RunE:  runCorrelate,
}

var correlateFlags struct {
	observables    string
	format         string
	workspace      string
	source         string
	allObservables bool
}

func init() {
	correlateCmd.Flags().StringVarP(&correlateFlags.observables, "observables", "O", "", "Case observables CSV file (type,value[,context])")
	correlateCmd.Flags().StringVar(&correlateFlags.format, "format", "text", "Output format (text, json)")
	correlateCmd.Flags().StringVar(&correlateFlags.workspace, "workspace", "", "Write a projected case workspace graph (.gob path, gob:<path>, or bolt:<path>)")
	correlateCmd.Flags().StringVar(&correlateFlags.source, "source", "case", "source_feed recorded on case observable nodes")
	correlateCmd.Flags().BoolVar(&correlateFlags.allObservables, "all-observables", false, "Materialize observables for every indicator, not just case-matched ones")
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
		stats, err := writeWorkspace(engine.Storage(), caseObs, correlateFlags.workspace, dfir.WorkspaceOptions{
			CaseSource:     correlateFlags.source,
			AllObservables: correlateFlags.allObservables,
		})
		if err != nil {
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

// writeWorkspace builds the case workspace from src and persists it to
// target: a bare path or gob:<path> writes gob via SaveGraph, while
// bolt:<path> creates a bolt store that is durable in-place (no save step).
func writeWorkspace(src portstorage.StorageEngine, caseObs []dfir.CaseObservable, target string, opts dfir.WorkspaceOptions) (dfir.WorkspaceStats, error) {
	var stats dfir.WorkspaceStats
	scheme, path := "gob", target
	if i := strings.Index(target, ":"); i > 0 {
		scheme, path = target[:i], target[i+1:]
	}
	switch scheme {
	case "gob":
		wsEngine := graph.NewGraphEngine(inmem.New())
		if err := wireExtensionVerbs(wsEngine); err != nil {
			return stats, err
		}
		var err error
		if stats, err = dfir.BuildWorkspace(src, wsEngine.Storage(), caseObs, opts); err != nil {
			return stats, err
		}
		if err := SaveGraph(wsEngine, path); err != nil {
			return stats, err
		}
		return stats, nil
	case "bolt":
		ensureDefaultStore()
		backend, err := portstorage.Default().Resolve(scheme + ":" + path)
		if err != nil {
			return stats, err
		}
		se, err := backend.Create(path)
		if err != nil {
			return stats, err
		}
		if c, ok := se.(interface{ Close() error }); ok {
			defer c.Close()
		}
		wsEngine := graph.NewGraphEngine(se)
		if err := wireExtensionVerbs(wsEngine); err != nil {
			return stats, err
		}
		if stats, err = dfir.BuildWorkspace(src, se, caseObs, opts); err != nil {
			return stats, err
		}
		return stats, nil // bolt is durable on write
	default:
		return stats, fmt.Errorf("unknown workspace scheme %q: want a .gob path, gob:<path>, or bolt:<path>", scheme)
	}
}
