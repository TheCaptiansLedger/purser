package theaudiodb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerify_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"artists":[{"strArtist":"Radiohead","strMusicBrainzID":"a74b1b7f-71a5-4011-9441-d0b5e4122711"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	if err := a.Verify(context.Background()); err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
}

func TestVerify_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error")) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
	err := a.Verify(context.Background())
	if err == nil {
		t.Fatal("Verify() = nil, want error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention 500, got: %v", err)
	}
}
