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
			"score": 0.98,
			"recordings": [
				{"id": "550e8400-e29b-41d4-a716-446655440000"},
				{"id": "660e9500-f30c-52e5-b827-557766551111"}
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
	mbids, err := client.Lookup(context.Background(), "raw-fingerprint", 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(mbids) != 2 {
		t.Fatalf("expected 2 MBIDs, got %d", len(mbids))
	}
	if mbids[0] != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("mbids[0] = %q", mbids[0])
	}
}

func TestAcoustIDClient_Lookup_NoResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(acoustidNoMatchResponse))
	}))
	defer srv.Close()

	client := identifier.NewTestAcoustIDClient("test-key", srv.URL)
	mbids, err := client.Lookup(context.Background(), "raw-fingerprint", 300)
	if err != nil {
		t.Fatal(err)
	}
	if len(mbids) != 0 {
		t.Errorf("expected 0 MBIDs, got %d", len(mbids))
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

	mbids1, err := client.Lookup(context.Background(), "fp-abc", 300)
	if err != nil {
		t.Fatal(err)
	}
	mbids2, err := client.Lookup(context.Background(), "fp-abc", 300)
	if err != nil {
		t.Fatal(err)
	}

	if calls.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", calls.Load())
	}
	if len(mbids1) != len(mbids2) {
		t.Errorf("cached result mismatch: first=%d second=%d", len(mbids1), len(mbids2))
	}
}
