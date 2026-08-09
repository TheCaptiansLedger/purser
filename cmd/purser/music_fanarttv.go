package main

import (
	"context"
	"fmt"
	"net/http"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	musicv1 "purser/gen/go/purser/music/v1"
)

// newMusicFanartTVCmd groups fanart.tv lookup subcommands — see
// newMusicCmd's own doc comment.
func newMusicFanartTVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "fanart",
		Aliases: []string{"fanarttv"},
		Short:   "Look up artist/album images from fanart.tv",
	}
	cmd.AddCommand(newMusicFanartTVLookupArtistCmd())
	return cmd
}

func newMusicFanartTVLookupArtistCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-artist <mbid>",
		Short: "Look up one artist's images (and every release group's album art) via FanartTVService",
		Long: "Calls purser.music.v1.FanartTVService.LookupArtist against a running\n" +
			"purser serve instance — one call returns both the artist's own images\n" +
			"and every one of their release groups' album art, keyed by\n" +
			"release-group MBID. See docs/adr/0027-provider-independence.md — this\n" +
			"is a read-only passthrough to fanart.tv's own data, never ranked or\n" +
			"merged against any other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := musicv1connect.NewFanartTVServiceClient(httpClient, addr)
			return runMusicFanartTVLookupArtist(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runMusicFanartTVLookupArtist calls client.LookupArtist and prints the
// result — split from newMusicFanartTVLookupArtistCmd's RunE closure so
// it's testable against a fake musicv1connect.FanartTVServiceClient, same
// convention runMusicTheAudioDBLookupArtist follows.
func runMusicFanartTVLookupArtist(ctx context.Context, client musicv1connect.FanartTVServiceClient, mbid string, jsonOutput bool) error {
	res, err := client.LookupArtist(ctx, connect.NewRequest(&musicv1.LookupFanartTVArtistRequest{Mbid: mbid}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, fanartTVArtistFields(res.Msg.GetArtist()))
}

// fanartTVArtistFields orders a's fields for printLookupResult's styled
// bullet-list path — image slots are summarized as counts (the full URLs
// are in every entry, better read via --json than crammed into one bullet
// line each); albums is summarized as a release-group count. --json still
// returns the complete response, every URL included.
func fanartTVArtistFields(a *musicv1.FanartTVArtist) []labeledField {
	return []labeledField{
		{"Name", a.GetName()},
		{"MBID", a.GetMbid()},
		{"Thumbnails", countLabel(len(a.GetArtistThumb()), "image(s)")},
		{"Backgrounds", countLabel(len(a.GetArtistBackground()), "image(s)")},
		{"4K Backgrounds", countLabel(len(a.GetArtist_4KBackground()), "image(s)")},
		{"HD Logos", countLabel(len(a.GetHdMusicLogo()), "image(s)")},
		{"Logos", countLabel(len(a.GetMusicLogo()), "image(s)")},
		{"Banners", countLabel(len(a.GetMusicBanner()), "image(s)")},
		{"Release Groups With Art", countLabel(len(a.GetAlbums()), "release group(s)")},
	}
}

// countLabel renders n as "N <unit>", or "" (skipped by
// printLookupResult's styled path) when n is zero — an artist missing a
// given image slot shouldn't clutter the bullet list with "0 image(s)".
func countLabel(n int, unit string) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d %s", n, unit)
}
