package github_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"purser/internal/adapters/github"
	"purser/pkg/cache"
	"testing"
)

const (
	fixtureIssues  = `[{"id":1,"number":42,"title":"Test issue","state":"open","labels":[],"user":{"login":"u","avatar_url":""},"comments":0,"updated_at":"2026-01-01T00:00:00Z","closed_at":null}]`
	fixtureRelease = `[{"id":1,"tag_name":"v0.1","name":"Release 0.1","html_url":"","published_at":"2026-01-01T00:00:00Z","body":""}]`
	fixtureCommits = `[{"sha":"abc","author":{"login":"alice","avatar_url":""}}]`
	fixtureCompare = `{"commits":[{"sha":"abc","author":{"login":"alice","avatar_url":""}}]}`
)

func newTestCache(t *testing.T) *cache.Cache {
	t.Helper()
	c, err := cache.New("test", 128)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func newTestAdapter(srv *httptest.Server, c *cache.Cache) *github.Adapter {
	return github.NewWithBaseURL(github.Config{Repo: "owner/repo"}, c, srv.URL)
}

func TestIssues_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureIssues))
	}))
	defer srv.Close()

	data, err := newTestAdapter(srv, newTestCache(t)).Issues(context.Background(), "open", "")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != fixtureIssues {
		t.Errorf("Issues() body = %s, want %s", data, fixtureIssues)
	}
}

func TestIssues_CacheHit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureIssues))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, newTestCache(t))
	for i := 0; i < 3; i++ {
		if _, err := a.Issues(context.Background(), "open", ""); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1 (cache should serve subsequent requests)", calls)
	}
}

func TestIssues_WithLabel(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureIssues))
	}))
	defer srv.Close()

	if _, err := newTestAdapter(srv, newTestCache(t)).Issues(context.Background(), "closed", "scope: epic"); err != nil {
		t.Fatal(err)
	}
	if gotQuery == "" {
		t.Fatal("expected query params, got empty string")
	}
}

func TestIssues_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv, newTestCache(t)).Issues(context.Background(), "open", "")
	if err == nil {
		t.Fatal("expected error for 403 rate limit response")
	}
}

func TestIssues_NonOKError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	_, err := newTestAdapter(srv, newTestCache(t)).Issues(context.Background(), "open", "")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestReleases_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureRelease))
	}))
	defer srv.Close()

	data, err := newTestAdapter(srv, newTestCache(t)).Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != fixtureRelease {
		t.Errorf("Releases() body = %s, want %s", data, fixtureRelease)
	}
}

func TestContributors_WithBase_ReturnsCommitsArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureCompare))
	}))
	defer srv.Close()

	data, err := newTestAdapter(srv, newTestCache(t)).Contributors(context.Background(), "v0.1", "v0.2")
	if err != nil {
		t.Fatal(err)
	}
	// proxy must extract the "commits" array from the compare wrapper
	if string(data) != fixtureCommits {
		t.Errorf("Contributors() = %s, want extracted commits array %s", data, fixtureCommits)
	}
}

func TestContributors_WithoutBase_ReturnsRawArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureCommits))
	}))
	defer srv.Close()

	data, err := newTestAdapter(srv, newTestCache(t)).Contributors(context.Background(), "", "v0.1")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != fixtureCommits {
		t.Errorf("Contributors() = %s, want %s", data, fixtureCommits)
	}
}

func TestContributors_CacheHit(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureCommits))
	}))
	defer srv.Close()

	a := newTestAdapter(srv, newTestCache(t))
	for i := 0; i < 3; i++ {
		if _, err := a.Contributors(context.Background(), "", "v0.1"); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Errorf("upstream called %d times, want 1", calls)
	}
}
