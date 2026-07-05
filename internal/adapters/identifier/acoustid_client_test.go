package identifier_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/identifier"
	"purser/pkg/cache"
	"sync/atomic"
	"testing"
)

const acoustidOKResponse = `{
	"status": "ok",
	"results": [
		{
			"id": "abc-acoustid-uuid",
			"score": 0.98,
			"recordings": [
				{
					"id": "550e8400-e29b-41d4-a716-446655440000",
					"title": "I Wish You Were There",
					"duration": 247.133,
					"artists": [{"name": "REO Speedwagon"}],
					"releases": [{"title": "Hi Infidelity (2024 Remaster)"}]
				},
				{
					"id": "660e9500-f30c-52e5-b827-557766551111",
					"title": "I Wish You Were There",
					"duration": 247.9,
					"artists": [{"name": "REO Speedwagon"}],
					"releases": [{"title": "Hi Infidelity"}]
				}
			]
		}
	]
}`

const acoustidNoMatchResponse = `{
	"status": "ok",
	"results": []
}`

const acoustidErrorResponse = `{
	"status": "error",
	"error": {"code": 4, "message": "invalid fingerprint"}
}`

func TestAcoustIDClient_Lookup_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(acoustidOKResponse))
	}))
	defer srv.Close()

	client := identifier.NewTestAcoustIDClient("test-key", srv.URL)
	matches, err := client.Lookup(context.Background(), "raw-fingerprint", 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.AcoustID != "abc-acoustid-uuid" {
		t.Errorf("AcoustID = %q, want abc-acoustid-uuid", m.AcoustID)
	}
	if m.Score != 0.98 {
		t.Errorf("Score = %f, want 0.98", m.Score)
	}
	if len(m.Recordings) != 2 {
		t.Fatalf("expected 2 recordings, got %d", len(m.Recordings))
	}
	if m.Recordings[0].MBID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("Recordings[0].MBID = %q", m.Recordings[0].MBID)
	}
	if m.Recordings[0].Title != "I Wish You Were There" {
		t.Errorf("Recordings[0].Title = %q", m.Recordings[0].Title)
	}
	if m.Recordings[0].Artist != "REO Speedwagon" {
		t.Errorf("Recordings[0].Artist = %q", m.Recordings[0].Artist)
	}
	if len(m.Recordings[0].Albums) != 1 || m.Recordings[0].Albums[0] != "Hi Infidelity (2024 Remaster)" {
		t.Errorf("Recordings[0].Albums = %v", m.Recordings[0].Albums)
	}
	// AcoustID returns fractional seconds; Duration must be truncated to int without error.
	if m.Recordings[0].Duration != 247 {
		t.Errorf("Recordings[0].Duration = %d, want 247", m.Recordings[0].Duration)
	}
}

func TestAcoustIDClient_Lookup_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(acoustidNoMatchResponse))
	}))
	defer srv.Close()

	client := identifier.NewTestAcoustIDClient("test-key", srv.URL)
	matches, err := client.Lookup(context.Background(), "raw-fingerprint", 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestAcoustIDClient_Lookup_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(acoustidErrorResponse))
	}))
	defer srv.Close()

	client := identifier.NewTestAcoustIDClient("test-key", srv.URL)
	_, err := client.Lookup(context.Background(), "bad-fingerprint", 0)
	if err == nil {
		t.Error("expected error for status=error response, got nil")
	}
}

func TestAcoustIDClient_Lookup_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := identifier.NewTestAcoustIDClient("test-key", srv.URL)
	_, err := client.Lookup(context.Background(), "fingerprint", 300)
	if err == nil {
		t.Error("expected error for HTTP 500, got nil")
	}
}

func TestAcoustIDClient_Lookup_CachesResult(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(acoustidOKResponse))
	}))
	defer srv.Close()

	c, err := cache.New("test-acoustid", 64)
	if err != nil {
		t.Fatal(err)
	}
	client := identifier.NewTestAcoustIDClientWithCache("test-key", srv.URL, c)

	matches1, err := client.Lookup(context.Background(), "fp-abc", 300)
	if err != nil {
		t.Fatal(err)
	}
	matches2, err := client.Lookup(context.Background(), "fp-abc", 300)
	if err != nil {
		t.Fatal(err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls.Load())
	}
	if len(matches1) != len(matches2) {
		t.Errorf("cached result mismatch: first=%d second=%d", len(matches1), len(matches2))
	}
}
