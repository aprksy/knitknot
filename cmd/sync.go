package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	portstorage "github.com/aprksy/knitknot/pkg/ports/storage"
)

// store: sync — ADR 0001 reserved sync surface (stub until a backend
// pair implements the merge).
func newSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "Sync graph data between storage backends (reserved)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fromURI, _ := cmd.Flags().GetString("from")
			if fromURI == "" {
				return fmt.Errorf("missing required --from flag (want --from <store-uri>)")
			}
			toURI, _ := cmd.Flags().GetString("to")
			if toURI == "" {
				return fmt.Errorf("missing required --to flag (want --to <store-uri>)")
			}
			ensureDefaultStore()
			fromScheme, fromPath, err := ResolveStoreURI(fromURI, "")
			if err != nil {
				return err
			}
			toScheme, toPath, err := ResolveStoreURI(toURI, "")
			if err != nil {
				return err
			}
			sel := portstorage.Default()
			fromBackend, err := sel.Resolve(fromURI)
			if err != nil {
				return err
			}
			toBackend, err := sel.Resolve(toURI)
			if err != nil {
				return err
			}
			if _, err := fromBackend.Open(fromPath); err != nil {
				return err
			}
			if _, err := toBackend.Open(toPath); err != nil {
				return err
			}
			return fmt.Errorf("sync not implemented for %s -> %s yet; ADR 0001 reserves this command surface", fromScheme, toScheme)
		},
	}
	c.Flags().String("from", "", "Source store URI (e.g. gob:file.gob)")
	c.Flags().String("to", "", "Destination store URI (e.g. gob:other.gob)")
	c.SilenceUsage = true
	return c
}

func init() {
	RootCmd.AddCommand(newSyncCmd())
}
