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

// newAfterDarkThePornDBCmd groups ThePornDB lookup subcommands — see
// newAfterDarkCmd's own doc comment.
func newAfterDarkThePornDBCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "theporndb",
		Aliases: []string{"tpdb"},
		Short:   "Look up a performer from ThePornDB",
	}
	cmd.AddCommand(newAfterDarkThePornDBLookupPerformerCmd())
	cmd.AddCommand(newAfterDarkThePornDBSearchPerformersCmd())
	return cmd
}

func newAfterDarkThePornDBLookupPerformerCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "lookup-performer <id>",
		Short: "Look up one performer's profile and images via ThePornDBService",
		Long: "Calls purser.afterdark.v1.ThePornDBService.LookupPerformer against a\n" +
			"running purser serve instance. See\n" +
			"docs/adr/0027-provider-independence.md — this is a read-only\n" +
			"passthrough to ThePornDB's own data, never ranked or merged against\n" +
			"any other provider.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := afterdarkv1connect.NewThePornDBServiceClient(httpClient, addr)
			return runAfterDarkThePornDBLookupPerformer(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runAfterDarkThePornDBLookupPerformer calls client.LookupPerformer and
// prints the result — split from newAfterDarkThePornDBLookupPerformerCmd's
// RunE closure so it's testable against a fake
// afterdarkv1connect.ThePornDBServiceClient, same convention
// runAfterDarkStashDBLookupPerformer follows.
func runAfterDarkThePornDBLookupPerformer(ctx context.Context, client afterdarkv1connect.ThePornDBServiceClient, id string, jsonOutput bool) error {
	res, err := client.LookupPerformer(ctx, connect.NewRequest(&afterdarkv1.LookupThePornDBPerformerRequest{Id: id}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, tpdbPerformerFields(res.Msg.GetPerformer()))
}

func newAfterDarkThePornDBSearchPerformersCmd() *cobra.Command {
	var addr string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "search-performers <term>",
		Short: "Free-text search performers by name/alias via ThePornDBService",
		Long: "Calls purser.afterdark.v1.ThePornDBService.SearchPerformers against a\n" +
			"running purser serve instance — the path a Person with no known\n" +
			"ThePornDB ID uses (#703). See docs/adr/0027-provider-independence.md\n" +
			"— results are printed exactly as ThePornDB ordered them, never\n" +
			"re-sorted.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			client := afterdarkv1connect.NewThePornDBServiceClient(httpClient, addr)
			return runAfterDarkThePornDBSearchPerformers(cmd.Context(), client, args[0], jsonOutput)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full response as machine-readable JSON instead of styled output")
	return cmd
}

// runAfterDarkThePornDBSearchPerformers calls client.SearchPerformers and
// prints the result — split from newAfterDarkThePornDBSearchPerformersCmd's
// RunE closure so it's testable against a fake
// afterdarkv1connect.ThePornDBServiceClient.
func runAfterDarkThePornDBSearchPerformers(ctx context.Context, client afterdarkv1connect.ThePornDBServiceClient, term string, jsonOutput bool) error {
	res, err := client.SearchPerformers(ctx, connect.NewRequest(&afterdarkv1.SearchThePornDBPerformersRequest{Term: term}))
	if err != nil {
		return err
	}
	return printLookupResult(res.Msg, jsonOutput, tpdbPerformerSearchFields(res.Msg.GetPerformers()))
}

// tpdbPerformerSearchFields orders every returned performer for
// printLookupResult's styled bullet-list path — one "Result N" bullet per
// performer, name and poster count only (the full detail is better read
// via --json).
func tpdbPerformerSearchFields(performers []*afterdarkv1.TPDBPerformer) []labeledField {
	fields := make([]labeledField, len(performers))
	for i, p := range performers {
		fields[i] = labeledField{
			fmt.Sprintf("Result %d", i+1),
			fmt.Sprintf("%s (id=%s, %s)", p.GetName(), p.GetId(), countLabel(len(p.GetPosters()), "poster(s)")),
		}
	}
	return fields
}

// tpdbPerformerFields orders p's fields for printLookupResult's styled
// bullet-list path — Posters is summarized as a count (the full URLs are
// in every entry, better read via --json than crammed into one bullet
// line each).
func tpdbPerformerFields(p *afterdarkv1.TPDBPerformer) []labeledField {
	return []labeledField{
		{"Name", p.GetName()},
		{"ID", p.GetId()},
		{"Image", p.GetImage()},
		{"Thumbnail", p.GetThumbnail()},
		{"Posters", countLabel(len(p.GetPosters()), "image(s)")},
	}
}
