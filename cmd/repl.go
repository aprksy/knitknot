package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/chzyer/readline"
	"github.com/spf13/cobra"
)

var replCmd = &cobra.Command{
	Use:   "repl",
	Short: "Start interactive KnitKnot shell",
	Long:  `Interactive mode to run and explain queries.`,
	RunE:  runRepl,
}

func init() {
	replCmd.Flags().StringVar(&globalFlags.subgraph, "subgraph", "", "Run query within a subgraph context")
	RootCmd.AddCommand(replCmd)
}

func runRepl(cmd *cobra.Command, args []string) error {
	// Setup readline
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          "knitknot> ",
		HistoryFile:     ".knitknot_history",
		AutoComplete:    nil,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		return err
	}
	defer rl.Close()

	fmt.Println("Welcome to KnitKnot REPL! Type a query or 'help'.")
	if globalFlags.file != "" {
		fmt.Printf("Using data file: %s\n", globalFlags.file)
	}
	fmt.Println("Press Ctrl+C to exit.")

	// Initialize engine
	// Load graph from -f
	engine, err := LoadGraph(globalFlags.file)
	if err != nil {
		return err
	}

	if globalFlags.subgraph != "" {
		engine = engine.WithSubgraph(globalFlags.subgraph)
	}

	ctx := context.Background()

	for {
		line, err := rl.Readline()
		if err != nil { // io.EOF or interrupt
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := handleLine(ctx, line, engine, rl.Stdout()); err != nil {
			fmt.Fprintf(rl.Stderr(), "Error: %v\n", err)
		}
	}

	// Autosave on exit if file was specified
	if globalFlags.file != "" {
		if err := SaveGraph(engine, globalFlags.file); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to save: %v\n", err)
		}
	}

	return nil
}

func handleLine(ctx context.Context, input string, engine *graph.GraphEngine, out io.Writer) error {
	var filename string
	input = strings.TrimSpace(input)
	lower := strings.ToLower(input)

	switch {
	case lower == "exit", lower == "quit":
		fmt.Fprintln(out, "Goodbye!")
		os.Exit(0)

	case lower == "help":
		printHelp(out)

	case strings.HasPrefix(lower, "explain "):
		return execExplain(ctx, input[8:], engine, out)

	case matchesCommand(lower, "save ", &filename):
		return execSave(engine, filename, out)

	case matchesCommand(lower, "load ", &filename):
		return execLoad(engine, filename, out)

	case strings.HasPrefix(lower, "define "):
		return execDefine(engine, input, out)

	case lower == "list verbs", lower == "verbs":
		return execListVerbs(engine, out)

	default:
		return execQuery(ctx, input, engine, out)
	}
	return nil
}

func printHelp(out io.Writer) {
	fmt.Fprintln(out, "KnitKnot REPL Commands:")
	fmt.Fprintln(out, "  Find('Label').Where(...)             - Run a query")
	fmt.Fprintln(out, "  EXPLAIN Find(...)                    - Show query plan")
	fmt.Fprintln(out, "  explain <query>                      - Same, case-insensitive")
	fmt.Fprintln(out, "  SAVE \"filename\"                      - Save graph to disk")
	fmt.Fprintln(out, "  LOAD \"filename\"                      - Load graph from disk")
	fmt.Fprintln(out, "  DEFINE <verb> TO <Label> VIA <prop>  - Register a relationship type")
	fmt.Fprintln(out, "  LIST VERBS                           - Show all defined verbs")
	fmt.Fprintln(out, "  VERBS                                - Short alias")
	fmt.Fprintln(out, "  help                                 - Show this message")
	fmt.Fprintln(out, "  exit / quit                          - Leave the shell")
	fmt.Fprintln(out, "")
}

// matchesCommand checks if input starts with cmd, and extracts quoted or unquoted arg
func matchesCommand(input, prefix string, result *string) bool {
	if !strings.HasPrefix(input, prefix) {
		return false
	}
	arg := strings.TrimSpace(input[len(prefix):])
	if arg == "" {
		return false
	}

	// Remove surrounding quotes if present
	unquoted := strings.Trim(arg, `"`)
	unquoted = strings.Trim(unquoted, `'`)
	*result = unquoted
	return true
}
