package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aprksy/knitknot/extensions/dfir"
)

var coverageCmd = &cobra.Command{
	Use:   "coverage",
	Short: "Measure case-evidence coverage of a CTI reference pattern",
	Long:  "Match DFIR case entities (CSV) against a CTI reference node's pattern and report per-dimension direct coverage plus inferred context (reachable but never counted).",
	RunE:  runCoverage,
}

var coverageFlags struct {
	observables string
	target      string
	format      string
}

func init() {
	coverageCmd.Flags().StringVarP(&coverageFlags.observables, "observables", "O", "", "Case entities CSV file (type,value[,context])")
	coverageCmd.Flags().StringVar(&coverageFlags.target, "target", "", "Reference node as <label>:<name>")
	coverageCmd.Flags().StringVar(&coverageFlags.format, "format", "text", "Output format (text, json)")
	RootCmd.AddCommand(coverageCmd)
}

func runCoverage(cmd *cobra.Command, args []string) error {
	if coverageFlags.observables == "" {
		return fmt.Errorf("usage: knitknot coverage --observables <case.csv> -f <graph.gob> --target <label>:<name>: missing --observables")
	}
	if coverageFlags.target == "" {
		return fmt.Errorf("usage: knitknot coverage --observables <case.csv> -f <graph.gob> --target <label>:<name>: missing --target")
	}
	label, name, ok := strings.Cut(coverageFlags.target, ":")
	if !ok || strings.TrimSpace(label) == "" || strings.TrimSpace(name) == "" {
		return fmt.Errorf("bad --target %q: want <label>:<name>", coverageFlags.target)
	}

	f, err := os.Open(coverageFlags.observables)
	if err != nil {
		return fmt.Errorf("open %s: %w", coverageFlags.observables, err)
	}
	defer f.Close()

	entities, err := dfir.ParseCaseEntities(f)
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

	target, err := dfir.ResolveTarget(engine.Storage(), label, name)
	if err != nil {
		return err
	}
	rep, err := dfir.Coverage(engine.Storage(), target, entities, dfir.DefaultCoverageDepth)
	if err != nil {
		return err
	}

	switch coverageFlags.format {
	case "text":
		fmt.Print(formatCoverageText(rep))
		return nil
	case "json":
		data, err := json.Marshal(rep)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	default:
		return fmt.Errorf("unknown format %q: want text or json", coverageFlags.format)
	}
}

// coveragePct renders a Covered/Total fraction; an empty pattern dimension
// is n/a, never 100%.
func coveragePct(covered, total int) string {
	if total == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%d%%", covered*100/total)
}

func formatCoverageText(rep dfir.CoverageReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "target: %s %s\n", rep.Target.Label, rep.Target.Name)
	for _, d := range rep.Dimensions {
		fmt.Fprintf(&b, "%s %d/%d %s\n", d.Label, d.Covered, d.Total, coveragePct(d.Covered, d.Total))
		fmt.Fprintf(&b, "  matched: %s\n", coverageNames(d.Matched))
		fmt.Fprintf(&b, "  unmatched: %s\n", coverageNames(d.Unmatched))
		if d.Label == "attack-pattern" {
			fmt.Fprintf(&b, "  %s\n", formatOrderedLine(d))
			if d.OrderedCoverage != nil {
				fmt.Fprintf(&b, "  expected: %s\n", coverageNames(d.ExpectedOrder))
				fmt.Fprintf(&b, "  actual: %s\n", coverageNames(d.ActualOrder))
			}
		}
	}
	fmt.Fprintf(&b, "Inferred context: %d nodes\n", len(rep.Inferred))
	return b.String()
}

// formatOrderedLine renders the attack-pattern sequence view: ordered
// LIS/N and its percent, or n/a with N when ranks or evidence are
// unavailable. LIS is recovered from the fraction by rounding, exact for
// any real N (float error is far below half a unit).
func formatOrderedLine(d dfir.DimensionCoverage) string {
	if d.OrderedCoverage == nil {
		return fmt.Sprintf("ordered n/a (N=%d)", d.OrderedN)
	}
	lis := int(math.Round(*d.OrderedCoverage * float64(d.OrderedN)))
	return fmt.Sprintf("ordered %d/%d %s", lis, d.OrderedN, coveragePct(lis, d.OrderedN))
}

func coverageNames(refs []dfir.NodeRef) string {
	if len(refs) == 0 {
		return "(none)"
	}
	names := make([]string, len(refs))
	for i, r := range refs {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}
