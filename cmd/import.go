package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/aprksy/knitknot/extensions"
	"github.com/aprksy/knitknot/extensions/cti"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{ // ext: import-cmd
	Use:   "import",                                                            // ext: import-cmd
	Short: "Import external data into the graph",                               // ext: import-cmd
	Long:  "Import data (e.g. STIX 2.1 bundles) into the current graph state.", // ext: import-cmd
	RunE:  runImport,                                                           // ext: import-cmd
}

var importFlags struct { // ext: import-cmd
	format string // ext: import-cmd
}

func init() { // ext: import-cmd
	importCmd.Flags().StringVar(&importFlags.format, "format", "", "Input format (e.g. stix)") // ext: import-cmd
	RootCmd.AddCommand(importCmd)                                                              // ext: import-cmd
}

// registerExtensions wires compiled-in extensions into the registry. // ext: import-cmd
// A later lane replaces the body with real registrations; runImport calls // ext: import-cmd
// through this hook so cmd/import.go stays untouched. // ext: import-cmd
var registerExtensions = func(r extension.Registry) error { // ext: cti-wire
	if err := cti.New().Register(r); err != nil { // ext: cti-wire
		return err // ext: cti-wire
	} // ext: cti-wire
	r.Freeze() // ext: cti-wire
	return nil // ext: cti-wire
}

func runImport(cmd *cobra.Command, args []string) error { // ext: import-cmd
	if importFlags.format == "" { // ext: import-cmd
		return fmt.Errorf("usage: knitknot import --format <name> -f <file.gob> <input>") // ext: import-cmd
	}
	if len(args) != 1 { // ext: import-cmd
		return fmt.Errorf("usage: knitknot import --format <name> -f <file.gob> <input>: missing input file") // ext: import-cmd
	}
	input := args[0] // ext: import-cmd

	engine, err := LoadGraph(globalFlags.file) // ext: import-cmd
	if err != nil {                            // ext: import-cmd
		return err // ext: import-cmd
	}
	if engine, err = resolveStoreBackend(engine); err != nil { // store: wire
		return err // store: wire
	}

	f, err := os.Open(input) // ext: import-cmd
	if err != nil {          // ext: import-cmd
		return fmt.Errorf("open %s: %w", input, err) // ext: import-cmd
	}
	defer f.Close() // ext: import-cmd

	reg := extension.NewRegistry()                  // ext: import-cmd
	if err := registerExtensions(reg); err != nil { // ext: import-cmd
		return err // ext: import-cmd
	}

	imp, ok := reg.Importer(importFlags.format) // ext: import-cmd
	if !ok {                                    // ext: import-cmd
		return fmt.Errorf("unknown import format: %s", importFlags.format) // ext: import-cmd
	}

	ctx := context.Background()             // ext: import-cmd
	if cmd != nil && cmd.Context() != nil { // ext: import-cmd
		ctx = cmd.Context() // ext: import-cmd
	}
	if err := imp.Import(ctx, engine.Storage(), engine.Verbs(), f); err != nil { // ext: import-cmd
		return fmt.Errorf("import %s as %s: %w", input, importFlags.format, err) // ext: import-cmd
	}

	if globalFlags.file != "" { // ext: import-cmd
		if err := SaveGraph(engine, globalFlags.file); err != nil { // ext: import-cmd
			return err // ext: import-cmd
		}
	}

	fmt.Printf("-- Imported %s as %s\n", input, importFlags.format) // ext: import-cmd
	return nil                                                      // ext: import-cmd
}
