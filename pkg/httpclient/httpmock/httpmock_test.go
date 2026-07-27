package httpmock_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"purser/pkg/httpclient/httpmock"
	"sync"
	"testing"
)

func doGet(t *testing.T, rt http.RoundTripper, url string) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	return rt.RoundTrip(req)
}

func TestTransport_JSONResponder_RepliesWithCannedBody(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/known",
		Responder: httpmock.JSON(http.StatusOK, payload{Name: "The Beatles"}),
	})

	resp, err := doGet(t, rt, "http://example.invalid/artist/known")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got payload
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response body: %v", err)
	}
	if got.Name != "The Beatles" {
		t.Errorf("Name = %q, want %q", got.Name, "The Beatles")
	}
}

func TestTransport_RawResponder_RepliesWithVerbatimBytes(t *testing.T) {
	recorded := []byte(`{"id":"b10bbbfc","name":"The Beatles"}`)
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/known",
		Responder: httpmock.Raw(http.StatusOK, recorded),
	})

	resp, err := doGet(t, rt, "http://example.invalid/artist/known")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	if string(body) != string(recorded) {
		t.Errorf("body = %q, want %q", body, recorded)
	}
}

func TestTransport_StatusResponder_RepliesWithEmptyBody(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/unknown",
		Responder: httpmock.Status(http.StatusNotFound),
	})

	resp, err := doGet(t, rt, "http://example.invalid/artist/unknown")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestTransport_ErrResponder_FailsRoundTrip(t *testing.T) {
	wantErr := errors.New("connection refused")
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/down",
		Responder: httpmock.Err(wantErr),
	})

	resp, err := doGet(t, rt, "http://example.invalid/artist/down")
	if resp != nil {
		defer resp.Body.Close()
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("RoundTrip error = %v, want %v", err, wantErr)
	}
}

func TestTransport_TimeoutResponder_ReturnsTimeoutError(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/slow",
		Responder: httpmock.Timeout(),
	})

	resp, err := doGet(t, rt, "http://example.invalid/artist/slow")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("RoundTrip returned nil error, want a timeout error")
	}
	var netErr interface{ Timeout() bool }
	if !errors.As(err, &netErr) {
		t.Fatalf("error %v does not implement Timeout() bool", err)
	}
	if !netErr.Timeout() {
		t.Errorf("Timeout() = false, want true")
	}
}

func TestTransport_UnmatchedRequest_ReturnsHardError(t *testing.T) {
	rt := httpmock.New(httpmock.Route{
		Method:    http.MethodGet,
		Path:      "/artist/known",
		Responder: httpmock.JSON(http.StatusOK, map[string]string{}),
	})

	resp, err := doGet(t, rt, "http://example.invalid/release/not-registered")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Fatal("RoundTrip returned nil error for an unregistered route, want a hard failure")
	}
}

func TestTransport_QueryMatching_SubsetMustMatch(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{
			Method:    http.MethodGet,
			Path:      "/artist",
			Query:     map[string]string{"query": "Beatles"},
			Responder: httpmock.JSON(http.StatusOK, map[string]string{"match": "yes"}),
		},
		httpmock.Route{
			Method:    http.MethodGet,
			Path:      "/artist",
			Responder: httpmock.JSON(http.StatusOK, map[string]string{"match": "no"}),
		},
	)

	resp, err := doGet(t, rt, "http://example.invalid/artist?query=Beatles&fmt=json")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()
	var got map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got["match"] != "yes" {
		t.Errorf("matched route = %v, want the query-specific route (extra fmt=json param must not block the match)", got)
	}

	resp2, err := doGet(t, rt, "http://example.invalid/artist?query=Someone+Else")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp2.Body.Close()
	var got2 map[string]string
	if err := json.NewDecoder(resp2.Body).Decode(&got2); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got2["match"] != "no" {
		t.Errorf("matched route = %v, want the fallback query-agnostic route", got2)
	}
}

func TestTransport_FirstMatchingRouteWins(t *testing.T) {
	rt := httpmock.New(
		httpmock.Route{Method: http.MethodGet, Path: "/x", Responder: httpmock.Status(http.StatusOK)},
		httpmock.Route{Method: http.MethodGet, Path: "/x", Responder: httpmock.Status(http.StatusTeapot)},
	)

	resp, err := doGet(t, rt, "http://example.invalid/x")
	if err != nil {
		t.Fatalf("RoundTrip returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want %d (first registered route)", resp.StatusCode, http.StatusOK)
	}
}

func TestTransport_Calls_RecordsEveryRequestInOrder(t *testing.T) {
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: "/x", Responder: httpmock.Status(http.StatusOK)})

	for range 3 {
		resp, err := doGet(t, rt, "http://example.invalid/x")
		if err != nil {
			t.Fatalf("RoundTrip returned error: %v", err)
		}
		resp.Body.Close()
	}

	calls := rt.Calls()
	if len(calls) != 3 {
		t.Fatalf("Calls() returned %d requests, want 3", len(calls))
	}
	for _, c := range calls {
		if c.URL.Path != "/x" {
			t.Errorf("recorded call path = %q, want /x", c.URL.Path)
		}
	}
}

func TestTransport_ConcurrentRoundTrips_AreRaceFree(t *testing.T) {
	rt := httpmock.New(httpmock.Route{Method: http.MethodGet, Path: "/x", Responder: httpmock.Status(http.StatusOK)})

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := doGet(t, rt, "http://example.invalid/x")
			if err != nil {
				t.Errorf("RoundTrip returned error: %v", err)
				return
			}
			resp.Body.Close()
		}()
	}
	wg.Wait()

	if len(rt.Calls()) != 20 {
		t.Errorf("Calls() returned %d requests, want 20", len(rt.Calls()))
	}
}
