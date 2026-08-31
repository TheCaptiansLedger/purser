package main

import (
	"context"
	"fmt"
	"net/http"
	"purser/gen/go/purser/afterdark/v1/afterdarkv1connect"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	afterdarkv1 "purser/gen/go/purser/afterdark/v1"
)

// newAfterDarkStashDBCmd groups StashDB lookup subcommands — see
// newAfterDarkCmd's own doc comment.
func newAfterDarkStashDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stashdb",
		Short: "Look up a performer from StashDB",
	}
	cmd.AddCommand(newAfterDarkStashDBLookupPerformerCmd())
	cmd.AddCommand(newAfterDarkStashDBSearchPerformersCmd())
	return cmd
}

func newAfterDarkStashDBLookupPerformerCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-performer <id>",
		Short: "Look up one performer's profile and images via StashDBService",
		Long: "Calls purser.afterdark.v1.StashDBService.LookupPerformer against a\n" +
			"running purser serve instance. See\n" +
			"docs/adr/0027-provider-independence.md — this is a read-only\n" +
			"passthrough to StashDB's own data, never ranked or merged against any\n" +
			"other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := afterdarkv1connect.NewStashDBServiceClient(httpClient, addr)
			return runAfterDarkStashDBLookupPerformer(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runAfterDarkStashDBLookupPerformer calls client.LookupPerformer and
// prints the result — split from newAfterDarkStashDBLookupPerformerCmd's
// RunE closure so it's testable against a fake
// afterdarkv1connect.StashDBServiceClient, same convention
// runMusicFanartTVLookupArtist follows.
func runAfterDarkStashDBLookupPerformer(ctx context.Context, client afterdarkv1connect.StashDBServiceClient, id string, jsonOutput bool) error {
	res, err := client.LookupPerformer(ctx, connect.NewRequest(&afterdarkv1.LookupStashDBPerformerRequest{Id: id}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, stashDBPerformerFields(res.Msg.GetPerformer()))
}

func newAfterDarkStashDBSearchPerformersCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "search-performers <term>",
		Short: "Free-text search performers by name/alias via StashDBService",
		Long: "Calls purser.afterdark.v1.StashDBService.SearchPerformers against a\n" +
			"running purser serve instance — the path a Person with no known\n" +
			"StashDB ID uses (#703). See docs/adr/0027-provider-independence.md —\n" +
			"results are printed exactly as StashDB ordered them, never re-sorted.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := afterdarkv1connect.NewStashDBServiceClient(httpClient, addr)
			return runAfterDarkStashDBSearchPerformers(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runAfterDarkStashDBSearchPerformers calls client.SearchPerformers and
// prints the result — split from newAfterDarkStashDBSearchPerformersCmd's
// RunE closure so it's testable against a fake
// afterdarkv1connect.StashDBServiceClient.
func runAfterDarkStashDBSearchPerformers(ctx context.Context, client afterdarkv1connect.StashDBServiceClient, term string, jsonOutput bool) error {
	res, err := client.SearchPerformers(ctx, connect.NewRequest(&afterdarkv1.SearchStashDBPerformersRequest{Term: term}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, stashDBPerformerSearchFields(res.Msg.GetPerformers()))
}

// stashDBPerformerSearchFields orders every returned performer for
// printLookupResult's styled bullet-list path — one "Result N" bullet per
// performer, name and image count only (the full detail is better read via
// --json).
func stashDBPerformerSearchFields(performers []*afterdarkv1.StashDBPerformer) []labeledField {
	fields := make([]labeledField, len(performers))
	for i, p := range performers {
		fields[i] = labeledField{
			fmt.Sprintf("Result %d", i+1),
			fmt.Sprintf("%s (id=%s, %s)", p.GetName(), p.GetId(), countLabel(len(p.GetImages()), "image(s)")),
		}
	}
	return fields
}

// stashDBPerformerFields orders p's fields for printLookupResult's styled
// bullet-list path — image/URL slots are summarized as counts (the full
// URLs are in every entry, better read via --json than crammed into one
// bullet line each).
func stashDBPerformerFields(p *afterdarkv1.StashDBPerformer) []labeledField {
	return []labeledField{
		{"Name", p.GetName()},
		{"ID", p.GetId()},
		{"Gender", p.GetGender()},
		{"Country", p.GetCountry()},
		{"Images", countLabel(len(p.GetImages()), "image(s)")},
		{"URLs", countLabel(len(p.GetUrls()), "url(s)")},
	}
}
