package cmd

// version: history-cmd — generic `history` command over VersionedStorage.

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/ports/storage"
	"github.com/aprksy/knitknot/pkg/ports/types"
)

// version: history-cmd — cobra wiring.
var historyCmd = &cobra.Command{
	Use:   "history <node-id>",
	Short: "Show version history for a node",
	RunE:  runHistory,
}

func init() {
	RootCmd.AddCommand(historyCmd)
}

// version: history-cmd — entrypoint: LoadGraph + store resolve + print.
func runHistory(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: knitknot history <node-id> -f <file.gob>")
	}
	id := args[0]

	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}
	if engine, err = resolveStoreBackend(engine); err != nil { // store: wire
		return err // store: wire
	}

	snaps, err := getHistory(engine, id)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if out == nil {
		out = os.Stdout
	}
	printNodeHistory(out, id, snaps)
	return nil
}

// version: history-cmd — backend resolution + empty-means-unknown guard.
func getHistory(engine *graph.GraphEngine, id string) ([]types.Snapshot, error) {
	vs, ok := engine.Storage().(storage.VersionedStorage)
	if !ok {
		return nil, fmt.Errorf("history not supported by this storage backend")
	}
	snaps := vs.GetNodeHistory(id)
	if len(snaps) == 0 {
		return nil, fmt.Errorf("no history for node %s", id)
	}
	return snaps, nil
}

// version: history-cmd — single-line-per-snapshot greppable printer, newest-last.
func printNodeHistory(out io.Writer, id string, snaps []types.Snapshot) {
	fmt.Fprintf(out, "-- History for %s (%d snapshots)\n", id, len(snaps))
	for _, s := range snaps {
		fmt.Fprintf(out, "rev=%d time=%s source=%s deleted=%t props=%s\n",
			s.Rev,
			s.EventTime.Format(time.RFC3339),
			s.Source,
			s.Deleted,
			formatHistoryProps(s.Props),
		)
	}
}

// version: history-cmd — sorted k=v props in {...}.
func formatHistoryProps(props map[string]any) string {
	if len(props) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%v", k, props[k]))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}
