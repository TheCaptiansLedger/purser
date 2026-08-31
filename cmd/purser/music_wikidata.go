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

// newMusicWikidataCmd groups Wikidata lookup subcommands — see
// newMusicCmd's own doc comment.
func newMusicWikidataCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wikidata",
		Short: "Look up a Person photo from Wikidata's P18 image claim",
	}
	cmd.AddCommand(newMusicWikidataLookupImageCmd())
	return cmd
}

func newMusicWikidataLookupImageCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-image <wikidata-url>",
		Short: "Resolve a wikidata.org entity URL's P18 image claim via WikidataService",
		Long: "Calls purser.music.v1.WikidataService.LookupImage against a running\n" +
			"purser serve instance — entity-url is a full wikidata.org entity URL\n" +
			"(e.g. https://www.wikidata.org/wiki/Q845084), the same URL\n" +
			"MusicBrainzService.GetArtist's own wikidata_url field surfaces from a\n" +
			"\"wikidata\" relation. See docs/adr/0027-provider-independence.md —\n" +
			"this is a read-only passthrough to Wikidata's own data, never ranked\n" +
			"or merged against any other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := musicv1connect.NewWikidataServiceClient(httpClient, addr)
			return runMusicWikidataLookupImage(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runMusicWikidataLookupImage calls client.LookupImage and prints the
// result — split from newMusicWikidataLookupImageCmd's RunE closure so
// it's testable against a fake musicv1connect.WikidataServiceClient, same
// convention runMusicFanartTVLookupArtist follows.
func runMusicWikidataLookupImage(ctx context.Context, client musicv1connect.WikidataServiceClient, entityURL string, jsonOutput bool) error {
	res, err := client.LookupImage(ctx, connect.NewRequest(&musicv1.LookupWikidataImageRequest{Url: entityURL}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, wikidataImageFields(res.Msg.GetImages()))
}

// wikidataImageFields orders the response's images for printLookupResult's
// styled bullet-list path — one bullet per resolved Commons file URL.
func wikidataImageFields(images []*musicv1.WikidataImage) []labeledField {
	fields := make([]labeledField, len(images))
	for i, img := range images {
		fields[i] = labeledField{"Image", img.GetUrl()}
	}
	return fields
}
