package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── library-entries ───────────────────────────────────────────────────────────

func TestProviderImages_Entry_EmptyArray(t *testing.T) {
	h := newHandler(t)
	entryID := piCreateEntry(t, h, "adult", "studio")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/library-entries/"+entryID+"/provider-images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body)
	}

	var images []map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&images); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if images == nil {
		t.Error("expected non-null JSON array, got null")
	}
	if len(images) != 0 {
		t.Errorf("got %d images, want 0 (no sources configured)", len(images))
	}
}

func TestProviderImages_Entry_NotFound(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/library-entries/does-not-exist/provider-images", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestProviderImages_Entry_ResponseShape(t *testing.T) {
	h := newHandler(t)
	entryID := piCreateEntry(t, h, "adult", "studio")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/library-entries/"+entryID+"/provider-images", nil))

	body := strings.TrimSpace(rec.Body.String())
	if body != "[]" {
		t.Errorf("body = %q, want %q", body, "[]")
	}
}

// ── groups ────────────────────────────────────────────────────────────────────

func TestProviderImages_Group_NotFound(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/does-not-exist/provider-images", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestProviderImages_Group_EmptyArray(t *testing.T) {
	h := newHandler(t)
	entryID := piCreateEntry(t, h, "music", "artist")
	groupID := piCreateGroup(t, h, entryID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/groups/"+groupID+"/provider-images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body)
	}

	var images []map[string]any
	json.NewDecoder(rec.Body).Decode(&images) //nolint:errcheck
	if images == nil {
		t.Error("expected non-null JSON array, got null")
	}
}

// ── items ─────────────────────────────────────────────────────────────────────

func TestProviderImages_Item_NotFound(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/does-not-exist/provider-images", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestProviderImages_Item_EmptyArray(t *testing.T) {
	h := newHandler(t)
	entryID := piCreateEntry(t, h, "adult", "studio")
	itemID := piCreateItem(t, h, entryID)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/items/"+itemID+"/provider-images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body)
	}

	var images []map[string]any
	json.NewDecoder(rec.Body).Decode(&images) //nolint:errcheck
	if images == nil {
		t.Error("expected non-null JSON array, got null")
	}
}

// ── people ────────────────────────────────────────────────────────────────────

func TestProviderImages_Person_NotFound(t *testing.T) {
	h := newHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/people/does-not-exist/provider-images", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestProviderImages_Person_EmptyArray(t *testing.T) {
	h := newHandler(t)
	personID := piCreatePerson(t, h)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/people/"+personID+"/provider-images", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body)
	}

	var images []map[string]any
	json.NewDecoder(rec.Body).Decode(&images) //nolint:errcheck
	if images == nil {
		t.Error("expected non-null JSON array, got null")
	}
}

// ── local test helpers ────────────────────────────────────────────────────────

func piCreateEntry(t *testing.T, h http.Handler, contentType, kind string) string {
	t.Helper()
	body := fmt.Sprintf(`{"contentType":%q,"kind":%q,"name":"PI Test","monitorMode":"all"}`, contentType, kind)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/library-entries", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create entry: status %d, body: %s", rec.Code, rec.Body)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode entry: %v", err)
	}
	return resp.ID
}

func piCreateGroup(t *testing.T, h http.Handler, entryID string) string {
	t.Helper()
	body := fmt.Sprintf(`{"libraryEntryId":%q,"title":"PI Test Album","monitored":true,"monitorMode":"all"}`, entryID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/groups", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create group: status %d, body: %s", rec.Code, rec.Body)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode group: %v", err)
	}
	return resp.ID
}

func piCreateItem(t *testing.T, h http.Handler, entryID string) string {
	t.Helper()
	body := fmt.Sprintf(`{"contentType":"adult","libraryEntryId":%q,"title":"PI Test Scene","monitored":true}`, entryID)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/items", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create item: status %d, body: %s", rec.Code, rec.Body)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode item: %v", err)
	}
	return resp.ID
}

func piCreatePerson(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/people", strings.NewReader(`{"name":"PI Test Person","monitorMode":"none"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create person: status %d, body: %s", rec.Code, rec.Body)
	}
	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode person: %v", err)
	}
	return resp.ID
}
