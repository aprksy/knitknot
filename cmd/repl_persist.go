package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/aprksy/knitknot/pkg/graph"
	"github.com/aprksy/knitknot/pkg/storage/inmem"
)

func execSave(engine *graph.GraphEngine, filename string, out io.Writer) error {
	storage, ok := engine.Storage().(*inmem.Storage)
	if !ok {
		return fmt.Errorf("storage does not support saving")
	}

	if filename == "" {
		return fmt.Errorf("missing filename")
	}

	if err := storage.Save(filename, engine); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}

	info, _ := os.Stat(filename)
	fmt.Fprintf(out, "-- Saved %d nodes, %d edges to %s (%.1f KB)\n",
		len(storage.GetAllNodes()),
		len(storage.GetAllEdges()),
		filename,
		float64(info.Size())/1024)
	return nil
}

func execLoad(engine *graph.GraphEngine, filename string, out io.Writer) error {
	storage, ok := engine.Storage().(*inmem.Storage)
	if !ok {
		return fmt.Errorf("storage does not support loading")
	}

	if filename == "" {
		return fmt.Errorf("missing filename")
	}

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", filename)
	}

	if err := storage.Load(filename, engine); err != nil {
		return fmt.Errorf("load failed: %w", err)
	}

	fmt.Fprintf(out, "-- Loaded %d nodes, %d edges from %s\n",
		len(storage.GetAllNodes()),
		len(storage.GetAllEdges()),
		filename)
	return nil
}
