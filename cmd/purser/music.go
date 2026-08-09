package main

import "github.com/spf13/cobra"

// newMusicCmd groups Music module provider-lookup subcommands — thin
// Connect clients against a running purser serve instance's read-only
// lookup RPCs (docs/adr/0027-provider-independence.md), same shape as
// jobs.go/seed_various_artists.go. Each provider gets its own child
// command rather than a flat top-level command per RPC, so a future
// provider (Last.fm, a manual MusicBrainz browse command, ...) is purely
// additive here.
func newMusicCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "music",
		Short: "Music module provider lookups (TheAudioDB, fanart.tv)",
	}
	cmd.AddCommand(newMusicTheAudioDBCmd())
	cmd.AddCommand(newMusicFanartTVCmd())
	return cmd
}
