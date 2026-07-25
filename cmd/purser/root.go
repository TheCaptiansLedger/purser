package main

import (
	"purser/internal/version"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "purser",
		Short:   "Purser media library manager",
		Version: version.String(),
	}
	cmd.AddCommand(newServeCmd())
	cmd.AddCommand(newSeedVariousArtistsCmd())
	cmd.AddCommand(newJobsCmd())
	return cmd
}
