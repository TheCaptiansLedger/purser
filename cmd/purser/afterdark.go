package main

import "github.com/spf13/cobra"

// newAfterDarkCmd groups AfterDark module provider-lookup subcommands —
// thin Connect clients against a running purser serve instance's
// read-only lookup RPCs (docs/adr/0027-provider-independence.md), same
// shape as newMusicCmd. Each provider gets its own child command rather
// than a flat top-level command per RPC, so a future provider is purely
// additive here.
func newAfterDarkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "afterdark",
		Short: "AfterDark module provider lookups (StashDB, ThePornDB)",
	}
	cmd.AddCommand(newAfterDarkStashDBCmd())
	cmd.AddCommand(newAfterDarkThePornDBCmd())
	return cmd
}
