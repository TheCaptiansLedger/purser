package main

import (
	"context"
	"net/http"
	"purser/gen/go/purser/music/v1/musicv1connect"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	musicv1 "purser/gen/go/purser/music/v1"
)

// newMusicTheAudioDBCmd groups TheAudioDB lookup subcommands — see
// newMusicCmd's own doc comment.
func newMusicTheAudioDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "theaudiodb",
		Short: "Look up artist/album images and metadata from TheAudioDB",
	}
	cmd.AddCommand(newMusicTheAudioDBLookupArtistCmd())
	cmd.AddCommand(newMusicTheAudioDBLookupAlbumCmd())
	return cmd
}

func newMusicTheAudioDBLookupArtistCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-artist <mbid>",
		Short: "Look up one artist by MusicBrainz artist MBID via TheAudioDBService",
		Long: "Calls purser.music.v1.TheAudioDBService.LookupArtist against a running\n" +
			"purser serve instance. See docs/adr/0027-provider-independence.md — this\n" +
			"is a read-only passthrough to TheAudioDB's own data, never ranked or\n" +
			"merged against any other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := musicv1connect.NewTheAudioDBServiceClient(httpClient, addr)
			return runMusicTheAudioDBLookupArtist(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runMusicTheAudioDBLookupArtist calls client.LookupArtist and prints the
// result — split from newMusicTheAudioDBLookupArtistCmd's RunE closure so
// it's testable against a fake musicv1connect.TheAudioDBServiceClient,
// same convention runSeedVariousArtists (seed_various_artists.go) already
// follows.
func runMusicTheAudioDBLookupArtist(ctx context.Context, client musicv1connect.TheAudioDBServiceClient, mbid string, jsonOutput bool) error {
	res, err := client.LookupArtist(ctx, connect.NewRequest(&musicv1.LookupTheAudioDBArtistRequest{Mbid: mbid}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, theAudioDBArtistFields(res.Msg.GetArtist()))
}

func newMusicTheAudioDBLookupAlbumCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-album <release-group-mbid>",
		Short: "Look up one album by MusicBrainz release-group MBID via TheAudioDBService",
		Long: "Calls purser.music.v1.TheAudioDBService.LookupAlbum against a running\n" +
			"purser serve instance. See docs/adr/0027-provider-independence.md — this\n" +
			"is a read-only passthrough to TheAudioDB's own data, never ranked or\n" +
			"merged against any other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := musicv1connect.NewTheAudioDBServiceClient(httpClient, addr)
			return runMusicTheAudioDBLookupAlbum(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runMusicTheAudioDBLookupAlbum mirrors runMusicTheAudioDBLookupArtist for
// LookupAlbum.
func runMusicTheAudioDBLookupAlbum(ctx context.Context, client musicv1connect.TheAudioDBServiceClient, releaseGroupMBID string, jsonOutput bool) error {
	res, err := client.LookupAlbum(ctx, connect.NewRequest(&musicv1.LookupTheAudioDBAlbumRequest{ReleaseGroupMbid: releaseGroupMBID}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, theAudioDBAlbumFields(res.Msg.GetAlbum()))
}

// theAudioDBArtistFields orders a's fields for printLookupResult's styled
// bullet-list path — a curated subset (identity + every image slot),
// skipping the locale/social fields that clutter a terminal glance; --json
// still returns the complete response.
func theAudioDBArtistFields(a *musicv1.TheAudioDBArtist) []labeledField {
	return []labeledField{
		{"Name", a.GetName()},
		{"MBID", a.GetMbid()},
		{"Genre", a.GetGenre()},
		{"Style", a.GetStyle()},
		{"Country", a.GetCountry()},
		{"Formed", a.GetFormedYear()},
		{"Disbanded", a.GetDisbanded()},
		{"Thumb", a.GetThumb()},
		{"Logo", a.GetLogo()},
		{"Cutout", a.GetCutout()},
		{"Clearart", a.GetClearart()},
		{"Wide Thumb", a.GetWideThumb()},
		{"Banner", a.GetBanner()},
		{"Fanart", a.GetFanart()},
		{"Fanart 2", a.GetFanart2()},
		{"Fanart 3", a.GetFanart3()},
		{"Fanart 4", a.GetFanart4()},
	}
}

// theAudioDBAlbumFields mirrors theAudioDBArtistFields for an album.
func theAudioDBAlbumFields(a *musicv1.TheAudioDBAlbum) []labeledField {
	return []labeledField{
		{"Title", a.GetTitle()},
		{"MBID", a.GetMbid()},
		{"Artist", a.GetArtistName()},
		{"Artist MBID", a.GetArtistMbid()},
		{"Year", a.GetYearReleased()},
		{"Genre", a.GetGenre()},
		{"Label", a.GetLabel()},
		{"Format", a.GetReleaseFormat()},
		{"Thumb", a.GetThumb()},
		{"Thumb (HQ)", a.GetThumbHq()},
		{"Back", a.GetBack()},
		{"CD Art", a.GetCdArt()},
		{"Spine", a.GetSpine()},
		{"3D Case", a.GetThreeDCase()},
		{"3D Flat", a.GetThreeDFlat()},
		{"3D Face", a.GetThreeDFace()},
		{"3D Thumb", a.GetThreeDThumb()},
	}
}
