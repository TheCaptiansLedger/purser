package stashdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"strings"
	"testing"
)

func TestVerify_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"searchStudio":[]}}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "valid"})
	if err := a.Verify(context.Background()); err != nil {
		t.Errorf("Verify() error = %v, want nil", err)
	}
}

func TestVerify_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized")) //nolint:errcheck
	}))
	defer srv.Close()

	a := stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "bad"})
	err := a.Verify(context.Background())
	if err == nil {
		t.Fatal("Verify() = nil, want error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401, got: %v", err)
	}
}
