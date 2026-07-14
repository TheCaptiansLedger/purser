package main

import (
	"context"
	"fmt"
	"net/http"
	"purser/gen/go/purser/domain/v1/domainv1connect"
	"purser/internal/domain"
	"time"

	"connectrpc.com/connect"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	v1 "purser/gen/go/purser/domain/v1"
)

// variousArtistsMBID is MusicBrainz's well-known artist record for VA
// compilations. See docs/adr/0021-music-domain-model.md's "Various Artists
// compilations" section: every VA compilation's Group.LibraryEntryID must
// point at the LibraryEntry seeded here, never be null.
const variousArtistsMBID = "89ad4ac3-39f7-470e-963a-56509c546377"

const variousArtistsName = "Various Artists"

func newSeedVariousArtistsCmd() *cobra.Command {
	var addr string

	cmd := &cobra.Command{
		Use:   "seed-various-artists",
		Short: "Idempotently seed the MusicBrainz \"Various Artists\" sentinel LibraryEntry",
		Long: "Get-or-creates a LibraryEntry{Kind: artist, Name: \"Various Artists\"} and\n" +
			"attaches an mbz ExternalID pointing at MusicBrainz's well-known Various\n" +
			"Artists record, per docs/adr/0021-music-domain-model.md. Safe to run\n" +
			"repeatedly against a live purser serve instance — a run after the first\n" +
			"successful one is a no-op.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			httpClient := &http.Client{Timeout: 30 * time.Second}
			libraryEntryClient := domainv1connect.NewLibraryEntryServiceClient(httpClient, addr)
			externalIDClient := domainv1connect.NewExternalIDServiceClient(httpClient, addr)
			return runSeedVariousArtists(cmd.Context(), libraryEntryClient, externalIDClient)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "http://localhost:7474", "base URL of a running purser serve instance")
	return cmd
}

// runSeedVariousArtists performs the get-or-create against entryClient,
// then the get-or-create against idClient, and prints the outcome.
func runSeedVariousArtists(ctx context.Context, entryClient domainv1connect.LibraryEntryServiceClient, idClient domainv1connect.ExternalIDServiceClient) error {
	entryID, entryCreated, err := findOrCreateVariousArtistsEntry(ctx, entryClient)
	if err != nil {
		return err
	}

	idCreated, err := findOrCreateVariousArtistsExternalID(ctx, idClient, entryID)
	if err != nil {
		return err
	}

	if entryCreated || idCreated {
		pterm.Success.Printfln("seeded Various Artists sentinel LibraryEntry %s (mbz external id -> %s)", entryID, variousArtistsMBID)
	} else {
		pterm.Info.Printfln("Various Artists sentinel LibraryEntry %s already seeded (mbz external id -> %s)", entryID, variousArtistsMBID)
	}
	return nil
}

// findOrCreateVariousArtistsEntry pages through every artist LibraryEntry
// looking for one named "Various Artists" and reuses it if found;
// otherwise it creates one.
func findOrCreateVariousArtistsEntry(ctx context.Context, client domainv1connect.LibraryEntryServiceClient) (id string, created bool, err error) {
	pageToken := ""
	for {
		resp, err := client.ListLibraryEntries(ctx, connect.NewRequest(&v1.ListLibraryEntriesRequest{
			Kind:      string(domain.KindArtist),
			PageToken: pageToken,
		}))
		if err != nil {
			return "", false, fmt.Errorf("listing library entries: %w", err)
		}
		for _, e := range resp.Msg.GetLibraryEntries() {
			if e.GetName() == variousArtistsName {
				return e.GetId(), false, nil
			}
		}
		pageToken = resp.Msg.GetNextPageToken()
		if pageToken == "" {
			break
		}
	}

	resp, err := client.CreateLibraryEntry(ctx, connect.NewRequest(&v1.CreateLibraryEntryRequest{
		LibraryEntry: &v1.LibraryEntry{
			ContentType: string(domain.ContentTypeMusic),
			Kind:        string(domain.KindArtist),
			Name:        variousArtistsName,
			MonitorMode: v1.MonitorMode_MONITOR_MODE_NONE,
		},
	}))
	if err != nil {
		return "", false, fmt.Errorf("creating various artists library entry: %w", err)
	}
	return resp.Msg.GetLibraryEntry().GetId(), true, nil
}

// findOrCreateVariousArtistsExternalID attaches the mbz ExternalID to
// entryID if one doesn't already exist. Get keys on the composite
// (entityType, entityID, source), so this is the idempotency guard for a
// re-run against an already-seeded entry.
func findOrCreateVariousArtistsExternalID(ctx context.Context, client domainv1connect.ExternalIDServiceClient, entryID string) (created bool, err error) {
	getResp, err := client.GetExternalID(ctx, connect.NewRequest(&v1.GetExternalIDRequest{
		EntityType: v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY,
		EntityId:   entryID,
		Source:     string(domain.ExternalIDSourceMBZ),
	}))
	switch {
	case err == nil:
		if got := getResp.Msg.GetExternalId().GetValue(); got != variousArtistsMBID {
			return false, fmt.Errorf("library entry %s already has an mbz external id %q, expected %q", entryID, got, variousArtistsMBID)
		}
		return false, nil
	case connect.CodeOf(err) != connect.CodeNotFound:
		return false, fmt.Errorf("getting various artists external id: %w", err)
	}

	_, err = client.CreateExternalID(ctx, connect.NewRequest(&v1.CreateExternalIDRequest{
		ExternalId: &v1.ExternalID{
			EntityType: v1.EntityType_ENTITY_TYPE_LIBRARY_ENTRY,
			EntityId:   entryID,
			Source:     string(domain.ExternalIDSourceMBZ),
			Value:      variousArtistsMBID,
		},
	}))
	if err != nil {
		return false, fmt.Errorf("creating various artists external id: %w", err)
	}
	return true, nil
}
