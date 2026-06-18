package stashdb_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/stashdb"
	"purser/internal/config"
	"purser/internal/domain"
	"strings"
	"testing"
)

func TestNew_UsesPublicURLDefault(t *testing.T) {
	a := stashdb.New(config.MetadataSourceConfig{APIKey: "key"})
	if a.Name() != "stashdb" {
		t.Errorf("Name() = %q, want stashdb", a.Name())
	}
}

func TestAdapter_Name(t *testing.T) {
	a := stashdb.New(config.MetadataSourceConfig{APIKey: "key"})
	if got := a.Name(); got != "stashdb" {
		t.Errorf("Name() = %q, want stashdb", got)
	}
}

func TestAdapter_ContentTypes(t *testing.T) {
	a := stashdb.New(config.MetadataSourceConfig{APIKey: "key"})
	types := a.ContentTypes()
	if len(types) != 2 {
		t.Fatalf("ContentTypes() len = %d, want 2", len(types))
	}
	want := map[domain.ContentType]bool{
		domain.ContentTypeAdult: true,
		domain.ContentTypeJAV:   true,
	}
	for _, ct := range types {
		if !want[ct] {
			t.Errorf("unexpected ContentType %q", ct)
		}
	}
}

func TestAdapter_gql_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized")) //nolint:errcheck
	}))
	defer srv.Close()

	a := stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "key"})
	_, err := a.SearchItems(context.Background(), domain.ContentTypeAdult, "query", 10)
	if err == nil {
		t.Fatal("expected error for non-200 HTTP response, got nil")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention HTTP 401, got: %v", err)
	}
}

func TestAdapter_gql_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors": [{"message": "permission denied"}]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "key"})
	_, err := a.SearchItems(context.Background(), domain.ContentTypeAdult, "query", 10)
	if err == nil {
		t.Fatal("expected error for GraphQL errors response, got nil")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error should contain GraphQL message, got: %v", err)
	}
}

func TestAdapter_gql_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{")) //nolint:errcheck
	}))
	defer srv.Close()

	a := stashdb.New(config.MetadataSourceConfig{URL: srv.URL, APIKey: "key"})
	_, err := a.SearchItems(context.Background(), domain.ContentTypeAdult, "query", 10)
	if err == nil {
		t.Fatal("expected error for invalid JSON body, got nil")
	}
}
