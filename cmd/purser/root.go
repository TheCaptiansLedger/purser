package main

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "purser",
		Short: "Purser media library manager",
	}
	cmd.AddCommand(newServeCmd())
	return cmd
}
