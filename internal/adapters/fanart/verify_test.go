package fanart_test

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
		w.Write([]byte(`{"name":"The Beatles","mbid_id":"b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d","artistthumb":[]}`)) //nolint:errcheck
	}))
	defer srv.Close()

	a := newTestAdapter(srv)
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

	a := newTestAdapter(srv)
	err := a.Verify(context.Background())
	if err == nil {
		t.Fatal("Verify() = nil, want error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error should mention 401, got: %v", err)
	}
}
