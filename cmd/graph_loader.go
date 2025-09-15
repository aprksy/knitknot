package cmd

import (
	"fmt"
	"os"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

// LoadGraph initializes the graph engine from file or creates new
func LoadGraph(filename string) (*graph.GraphEngine, error) {
	storage := inmem.New()
	engine := graph.NewGraphEngine(storage)

	if filename == "" {
		return engine, nil
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "File %s not found, starting with empty graph\n", filename)
		return engine, nil
	}

	if err := storage.Load(filename, engine); err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", filename, err)
	}

	fmt.Fprintf(os.Stderr, "-- Loaded %d nodes, %d edges from %s\n",
		len(storage.GetAllNodes()),
		len(storage.GetAllEdges()),
		filename)

	return engine, nil
}

// SaveGraph saves the engine's graph to file
func SaveGraph(engine *graph.GraphEngine, filename string) error {
	storage, ok := engine.Storage().(*inmem.Storage)
	if !ok {
		return fmt.Errorf("storage does not support saving")
	}

	if err := storage.Save(filename, engine); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	info, _ := os.Stat(filename)
	fmt.Fprintf(os.Stderr, "-- Saved to %s (%.1f KB)\n",
		filename,
		float64(info.Size())/1024)

	return nil
}
