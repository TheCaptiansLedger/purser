package api_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"purser/internal/api"
	"purser/internal/app/library"
	"purser/internal/app/metadata"
	"purser/internal/app/people"
	"purser/internal/config"
	"purser/internal/ports"
	"purser/web"
	"testing"
	"time"

	dbadapter "purser/internal/adapters/db"
	jobsadapter "purser/internal/adapters/jobs"
)

// newHandlerWithDB builds a full server backed by a temp-file SQLite database
// and returns both the HTTP handler and the underlying database for test setup.
func newHandlerWithDB(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	database, err := dbadapter.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	personRepo := dbadapter.NewPersonRepo(database)
	libSvc := library.New(
		dbadapter.NewLibraryEntryRepo(database),
		dbadapter.NewGroupRepo(database),
		dbadapter.NewItemRepo(database),
		personRepo,
	)
	peopleSvc := people.New(personRepo)
	tagRepo := dbadapter.NewTagRepo(database)

	jobQueue := jobsadapter.New(1)
	t.Cleanup(jobQueue.Close)

	metaSvc := metadata.New(nil, jobQueue, dbadapter.NewLibraryEntryRepo(database), dbadapter.NewItemRepo(database), dbadapter.NewPersonRepo(database), dbadapter.NewTagRepo(database), dbadapter.NewExternalIDRepo(database), "")

	uiFS, _ := fs.Sub(web.Dist, "dist")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 0, Workers: 1},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: dbPath},
		Media:    config.MediaConfig{Path: ""},
		Modules: config.ModulesConfig{
			Movies:    config.ModuleConfig{Enabled: true},
			TV:        config.ModuleConfig{Enabled: true},
			Music:     config.ModuleConfig{Enabled: true},
			Books:     config.ModuleConfig{Enabled: true},
			AfterDark: config.ModuleConfig{Enabled: true},
			JAV:       config.ModuleConfig{Enabled: true},
		},
		Log: config.LogConfig{Level: "info", Format: "text"},
	}
	return api.New(0, "", cfg, database, libSvc, peopleSvc, metaSvc, tagRepo, jobQueue, uiFS).Handler(), database
}

// newHandler builds a full server backed by a temp-file SQLite database.
// A temp file is used (not :memory:) because SQLite creates a separate DB per
// connection for :memory:, which breaks nested queries that use the pool.
func newHandler(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newHandlerWithDB(t)
	return h
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v — body: %s", err, w.Body.String())
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

func TestConfig_Get(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/api/v1/config", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Server struct {
			Port int `json:"port"`
		} `json:"server"`
		Database struct {
			Driver string `json:"driver"`
			DSN    string `json:"dsn"`
		} `json:"database"`
		Modules struct {
			Movies struct {
				Enabled bool `json:"enabled"`
			} `json:"movies"`
			AfterDark struct {
				Enabled bool `json:"enabled"`
			} `json:"afterdark"`
		} `json:"modules"`
		Log struct {
			Level string `json:"level"`
		} `json:"log"`
	}
	decodeJSON(t, w, &resp)
	if resp.Server.Port != 0 {
		t.Errorf("port = %d, want 0", resp.Server.Port)
	}
	if resp.Database.Driver != "sqlite" {
		t.Errorf("driver = %q, want sqlite", resp.Database.Driver)
	}
	if !resp.Modules.Movies.Enabled {
		t.Error("movies should be enabled")
	}
	if !resp.Modules.AfterDark.Enabled {
		t.Error("afterdark should be enabled")
	}
	if resp.Log.Level != "info" {
		t.Errorf("log level = %q, want info", resp.Log.Level)
	}
}

// ── Library entries ───────────────────────────────────────────────────────────

func TestLibraryEntries_ListEmpty(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/api/v1/library-entries", nil)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data  []any `json:"data"`
		Total int   `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if len(resp.Data) != 0 {
		t.Errorf("data len = %d, want 0", len(resp.Data))
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

func TestLibraryEntries_Create_Valid(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult",
		"kind":        "studio",
		"name":        "Evil Angel",
		"monitored":   true,
	})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 — body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID          string `json:"id"`
		ContentType string `json:"contentType"`
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		SortName    string `json:"sortName"`
		MonitorMode string `json:"monitorMode"`
		Status      string `json:"status"`
		Monitored   bool   `json:"monitored"`
		ExternalIDs []any  `json:"externalIds"`
		Tags        []any  `json:"tags"`
	}
	decodeJSON(t, w, &resp)

	if resp.ID == "" {
		t.Error("id should be set")
	}
	if resp.ContentType != "adult" {
		t.Errorf("contentType = %q, want adult", resp.ContentType)
	}
	if resp.SortName != "Evil Angel" {
		t.Errorf("sortName = %q, want Evil Angel (defaulted from name)", resp.SortName)
	}
	if resp.MonitorMode != "all" {
		t.Errorf("monitorMode = %q, want all", resp.MonitorMode)
	}
	if resp.Status != "active" {
		t.Errorf("status = %q, want active", resp.Status)
	}
	if !resp.Monitored {
		t.Error("monitored should be true")
	}
	if resp.ExternalIDs == nil {
		t.Error("externalIds should be [] not null")
	}
}

func TestLibraryEntries_Create_ValidationError(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult",
		"kind":        "studio",
		// name missing
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "VALIDATION_ERROR" {
		t.Errorf("code = %q, want VALIDATION_ERROR", resp.Code)
	}
}

func TestLibraryEntries_Create_InvalidContentType(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "unknown",
		"kind":        "studio",
		"name":        "Test",
	})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestLibraryEntries_Get_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/api/v1/library-entries/no-such-id", nil)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", resp.Code)
	}
}

func TestLibraryEntries_CreateAndGet(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv",
		"kind":        "series",
		"name":        "Breaking Bad",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d", w.Code)
	}
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	w = do(t, h, http.MethodGet, "/api/v1/library-entries/"+created.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get status = %d", w.Code)
	}
	var got struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	decodeJSON(t, w, &got)
	if got.ID != created.ID {
		t.Errorf("id mismatch: got %q", got.ID)
	}
	if got.Name != "Breaking Bad" {
		t.Errorf("name = %q, want Breaking Bad", got.Name)
	}
}

func TestLibraryEntries_Patch(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "Studio A",
	})
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	w = do(t, h, http.MethodPatch, "/api/v1/library-entries/"+created.ID, map[string]any{
		"monitored": true,
		"overview":  "Great studio",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body: %s", w.Code, w.Body.String())
	}
	var patched struct {
		Monitored bool   `json:"monitored"`
		Overview  string `json:"overview"`
	}
	decodeJSON(t, w, &patched)
	if !patched.Monitored {
		t.Error("monitored should be true after patch")
	}
	if patched.Overview != "Great studio" {
		t.Errorf("overview = %q, want Great studio", patched.Overview)
	}
}

func TestLibraryEntries_Delete(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "Delete Me",
	})
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	w = do(t, h, http.MethodDelete, "/api/v1/library-entries/"+created.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", w.Code)
	}

	w = do(t, h, http.MethodGet, "/api/v1/library-entries/"+created.ID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", w.Code)
	}
}

func TestLibraryEntries_List_FilterByContentType(t *testing.T) {
	h := newHandler(t)

	for _, ct := range []string{"adult", "adult", "tv"} {
		do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
			"contentType": ct, "kind": "studio", "name": "Entry " + ct,
		})
	}

	w := do(t, h, http.MethodGet, "/api/v1/library-entries?contentType=adult", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 2 {
		t.Errorf("adult total = %d, want 2", resp.Total)
	}
}

func TestLibraryEntries_Children(t *testing.T) {
	h := newHandler(t)

	// Create a network
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "network", "name": "Network A",
	})
	var network struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &network)

	// Create two studios under it
	for _, name := range []string{"Studio 1", "Studio 2"} {
		do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
			"contentType": "adult", "kind": "studio", "name": name, "parentId": network.ID,
		})
	}

	w = do(t, h, http.MethodGet, "/api/v1/library-entries/"+network.ID+"/children", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("children status = %d", w.Code)
	}
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 2 {
		t.Errorf("children total = %d, want 2", resp.Total)
	}
}

// ── Items ─────────────────────────────────────────────────────────────────────

func TestItems_Create_InheritsContentType(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "Test Studio",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID,
		"title":          "Test Scene",
		"date":           "2024-03-15",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create item status = %d, body: %s", w.Code, w.Body.String())
	}
	var item struct {
		ContentType string `json:"contentType"`
		Date        string `json:"date"`
		Status      string `json:"status"`
	}
	decodeJSON(t, w, &item)
	if item.ContentType != "adult" {
		t.Errorf("contentType = %q, want adult (inherited)", item.ContentType)
	}
	if item.Date != "2024-03-15" {
		t.Errorf("date = %q, want 2024-03-15", item.Date)
	}
	if item.Status != "wanted" {
		t.Errorf("status = %q, want wanted", item.Status)
	}
}

func TestItems_Patch_Monitored(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene", "monitored": true,
	})
	var item struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &item)

	w = do(t, h, http.MethodPatch, "/api/v1/items/"+item.ID, map[string]any{
		"monitored": false,
		"status":    "skipped",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d", w.Code)
	}
	var patched struct {
		Monitored bool   `json:"monitored"`
		Status    string `json:"status"`
	}
	decodeJSON(t, w, &patched)
	if patched.Monitored {
		t.Error("monitored should be false after patch")
	}
	if patched.Status != "skipped" {
		t.Errorf("status = %q, want skipped", patched.Status)
	}
}

func TestItems_ListByLibraryEntry(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	for _, title := range []string{"Scene 1", "Scene 2", "Scene 3"} {
		do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
			"libraryEntryId": studio.ID, "title": title,
		})
	}

	w = do(t, h, http.MethodGet, "/api/v1/items?libraryEntryId="+studio.ID, nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 3 {
		t.Errorf("item total = %d, want 3", resp.Total)
	}
}

func TestItems_List_SortByTitle(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	for _, title := range []string{"Zebra Scene", "Apple Scene", "Mango Scene"} {
		do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
			"libraryEntryId": studio.ID, "title": title,
		})
	}

	w = do(t, h, http.MethodGet, "/api/v1/items?libraryEntryId="+studio.ID+"&sort=title&sortDir=asc", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data []struct {
			Title string `json:"title"`
		} `json:"data"`
	}
	decodeJSON(t, w, &resp)
	if len(resp.Data) != 3 {
		t.Fatalf("data len = %d, want 3", len(resp.Data))
	}
	if resp.Data[0].Title != "Apple Scene" {
		t.Errorf("first item = %q, want Apple Scene", resp.Data[0].Title)
	}
	if resp.Data[2].Title != "Zebra Scene" {
		t.Errorf("last item = %q, want Zebra Scene", resp.Data[2].Title)
	}
}

// ── People ────────────────────────────────────────────────────────────────────

func TestPeople_Create_WithAliases(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/people", map[string]any{
		"name":      "Jane Doe",
		"aliases":   []string{"J. Doe", "JD"},
		"monitored": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Name      string   `json:"name"`
		SortName  string   `json:"sortName"`
		Aliases   []string `json:"aliases"`
		Monitored bool     `json:"monitored"`
	}
	decodeJSON(t, w, &resp)
	if resp.SortName != "Jane Doe" {
		t.Errorf("sortName = %q, want Jane Doe", resp.SortName)
	}
	if len(resp.Aliases) != 2 {
		t.Errorf("aliases count = %d, want 2", len(resp.Aliases))
	}
	if !resp.Monitored {
		t.Error("monitored should be true")
	}
}

func TestPeople_Patch(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Alice"})
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	w = do(t, h, http.MethodPatch, "/api/v1/people/"+created.ID, map[string]any{
		"monitored": true,
		"overview":  "Famous performer",
		"aliases":   []string{"Ali"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d", w.Code)
	}
	var patched struct {
		Monitored bool     `json:"monitored"`
		Aliases   []string `json:"aliases"`
	}
	decodeJSON(t, w, &patched)
	if !patched.Monitored {
		t.Error("monitored should be true after patch")
	}
	if len(patched.Aliases) != 1 || patched.Aliases[0] != "Ali" {
		t.Errorf("aliases = %v, want [Ali]", patched.Aliases)
	}
}

func TestPeople_SearchByAlias(t *testing.T) {
	h := newHandler(t)

	do(t, h, http.MethodPost, "/api/v1/people", map[string]any{
		"name": "Jane Doe", "aliases": []string{"JD"},
	})
	do(t, h, http.MethodPost, "/api/v1/people", map[string]any{
		"name": "Bob Smith",
	})

	w := do(t, h, http.MethodGet, "/api/v1/people?search=JD", nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("search JD total = %d, want 1", resp.Total)
	}
}

// ── Tags ──────────────────────────────────────────────────────────────────────

func TestTags_CreateAndList(t *testing.T) {
	h := newHandler(t)

	for _, name := range []string{"blonde", "MILF", "favourite"} {
		scope := "metadata"
		if name == "favourite" {
			scope = "user"
		}
		w := do(t, h, http.MethodPost, "/api/v1/tags", map[string]any{
			"name": name, "scope": scope,
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("create tag %q status = %d", name, w.Code)
		}
	}

	w := do(t, h, http.MethodGet, "/api/v1/tags", nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}

	w = do(t, h, http.MethodGet, "/api/v1/tags?scope=user", nil)
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("user tags total = %d, want 1", resp.Total)
	}
}

func TestTags_EmptyName_Rejected(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/tags", map[string]any{"name": ""})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

// ── Groups ────────────────────────────────────────────────────────────────────

func TestGroups_CreateAndList(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "Breaking Bad",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)

	for i := 1; i <= 5; i++ {
		do(t, h, http.MethodPost, "/api/v1/groups", map[string]any{
			"libraryEntryId": series.ID,
			"title":          "Season " + string(rune('0'+i)),
			"number":         i,
		})
	}

	w = do(t, h, http.MethodGet, "/api/v1/groups?libraryEntryId="+series.ID, nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 5 {
		t.Errorf("groups total = %d, want 5", resp.Total)
	}
}

// ── Web UI ────────────────────────────────────────────────────────────────────

func TestWebUI_ServesPlaceholder(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("UI status = %d, want 200", w.Code)
	}
	if body := w.Body.String(); len(body) == 0 {
		t.Error("UI body should not be empty")
	}
}

func TestWebUI_UnknownPathFallsBackToIndex(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/library/some-unknown-route", nil)
	// Should serve index.html (200) for SPA routing
	if w.Code != http.StatusOK {
		t.Fatalf("SPA fallback status = %d, want 200", w.Code)
	}
}

// ── Additional handler coverage ───────────────────────────────────────────────

func TestGroups_GetAndPatchAndDelete(t *testing.T) {
	h := newHandler(t)

	// Create series
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "Breaking Bad",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)

	// Create group
	w = do(t, h, http.MethodPost, "/api/v1/groups", map[string]any{
		"libraryEntryId": series.ID, "title": "Season 1", "number": 1, "monitored": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create group status = %d, body: %s", w.Code, w.Body.String())
	}
	var group struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &group)

	// GET /:id
	w = do(t, h, http.MethodGet, "/api/v1/groups/"+group.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get group status = %d", w.Code)
	}
	var got struct {
		Title  string `json:"title"`
		Number int    `json:"number"`
	}
	decodeJSON(t, w, &got)
	if got.Title != "Season 1" || got.Number != 1 {
		t.Errorf("get group: title=%q number=%d", got.Title, got.Number)
	}

	// PATCH
	w = do(t, h, http.MethodPatch, "/api/v1/groups/"+group.ID, map[string]any{
		"title": "Season One", "monitored": false,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch group status = %d", w.Code)
	}
	var patched struct {
		Title     string `json:"title"`
		Monitored bool   `json:"monitored"`
	}
	decodeJSON(t, w, &patched)
	if patched.Title != "Season One" {
		t.Errorf("patched title = %q, want Season One", patched.Title)
	}
	if patched.Monitored {
		t.Error("monitored should be false after patch")
	}

	// DELETE
	w = do(t, h, http.MethodDelete, "/api/v1/groups/"+group.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete group status = %d, want 204", w.Code)
	}

	// GET after delete → 404
	w = do(t, h, http.MethodGet, "/api/v1/groups/"+group.ID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", w.Code)
	}
}

func TestItems_GetAndDelete(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene X",
	})
	var item struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &item)

	// GET /:id
	w = do(t, h, http.MethodGet, "/api/v1/items/"+item.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get item = %d", w.Code)
	}
	var got struct {
		Title string `json:"title"`
	}
	decodeJSON(t, w, &got)
	if got.Title != "Scene X" {
		t.Errorf("title = %q, want Scene X", got.Title)
	}

	// DELETE
	w = do(t, h, http.MethodDelete, "/api/v1/items/"+item.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete item = %d, want 204", w.Code)
	}
	w = do(t, h, http.MethodGet, "/api/v1/items/"+item.ID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", w.Code)
	}
}

func TestPeople_GetAndDelete(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Test Person"})
	var person struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &person)

	// GET /:id
	w = do(t, h, http.MethodGet, "/api/v1/people/"+person.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get person = %d", w.Code)
	}

	// DELETE
	w = do(t, h, http.MethodDelete, "/api/v1/people/"+person.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete person = %d, want 204", w.Code)
	}
	w = do(t, h, http.MethodGet, "/api/v1/people/"+person.ID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete = %d, want 404", w.Code)
	}
}

func TestTags_Delete(t *testing.T) {
	h := newHandler(t)

	w := do(t, h, http.MethodPost, "/api/v1/tags", map[string]any{"name": "blonde", "scope": "metadata"})
	var tag struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &tag)

	w = do(t, h, http.MethodDelete, "/api/v1/tags/"+tag.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete tag = %d, want 204", w.Code)
	}
}

// ── Bad-JSON 400 paths ────────────────────────────────────────────────────────

func TestLibraryEntries_Create_BadJSON(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/library-entries", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestLibraryEntries_Update_BadJSON(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/library-entries/"+created.ID, bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGroups_Create_BadJSON(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/groups", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestGroups_Update_BadJSON(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "BB",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)
	w = do(t, h, http.MethodPost, "/api/v1/groups", map[string]any{
		"libraryEntryId": series.ID, "title": "S1", "number": 1,
	})
	var group struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &group)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/groups/"+group.ID, bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestItems_Create_BadJSON(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/items", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestItems_Update_BadJSON(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)
	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene",
	})
	var item struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &item)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/items/"+item.ID, bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPeople_Create_BadJSON(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/people", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestPeople_Update_BadJSON(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Alice"})
	var person struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &person)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/people/"+person.ID, bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestTags_Create_BadJSON(t *testing.T) {
	h := newHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tags", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

// ── boolPtr / paginate edge cases ─────────────────────────────────────────────

func TestList_Monitored_InvalidBool(t *testing.T) {
	h := newHandler(t)
	// "notabool" is not parseable — boolPtr should return nil, list returns all
	w := do(t, h, http.MethodGet, "/api/v1/library-entries?monitored=notabool", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

func TestList_Pagination(t *testing.T) {
	h := newHandler(t)
	for i := 0; i < 5; i++ {
		do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
			"contentType": "adult", "kind": "studio", "name": "Studio",
		})
	}

	w := do(t, h, http.MethodGet, "/api/v1/library-entries?limit=2&offset=0", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data   []any `json:"data"`
		Total  int   `json:"total"`
		Limit  int   `json:"limit"`
		Offset int   `json:"offset"`
	}
	decodeJSON(t, w, &resp)
	if len(resp.Data) != 2 {
		t.Errorf("data len = %d, want 2", len(resp.Data))
	}
	if resp.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Total)
	}
	if resp.Limit != 2 {
		t.Errorf("limit = %d, want 2", resp.Limit)
	}
}

// ── ExternalIDs in responses ──────────────────────────────────────────────────

func TestLibraryEntries_Create_WithExternalIDs(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult",
		"kind":        "studio",
		"name":        "Test Studio",
		"externalIds": []map[string]any{
			{"source": "stashdb", "value": "abc-123"},
		},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ExternalIDs []struct {
			Source string `json:"source"`
			Value  string `json:"value"`
		} `json:"externalIds"`
	}
	decodeJSON(t, w, &resp)
	if len(resp.ExternalIDs) != 1 {
		t.Errorf("externalIds count = %d, want 1", len(resp.ExternalIDs))
	}
	if resp.ExternalIDs[0].Value != "abc-123" {
		t.Errorf("externalId value = %q, want abc-123", resp.ExternalIDs[0].Value)
	}
}

func TestGroups_Create_WithExternalIDs(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "BB",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)

	w = do(t, h, http.MethodPost, "/api/v1/groups", map[string]any{
		"libraryEntryId": series.ID,
		"title":          "Season 1",
		"number":         1,
		"externalIds":    []map[string]any{{"source": "tvdb", "value": "s1-id"}},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ExternalIDs []struct {
			Value string `json:"value"`
		} `json:"externalIds"`
	}
	decodeJSON(t, w, &resp)
	if len(resp.ExternalIDs) != 1 || resp.ExternalIDs[0].Value != "s1-id" {
		t.Errorf("externalIds = %v, want [{s1-id}]", resp.ExternalIDs)
	}
}

// ── Extended PATCH field coverage ─────────────────────────────────────────────

func TestLibraryEntries_Patch_AllFields(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "Original",
	})
	var created struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &created)

	w = do(t, h, http.MethodPatch, "/api/v1/library-entries/"+created.ID, map[string]any{
		"name":        "Renamed",
		"sortName":    "Renamed Sort",
		"status":      "ended",
		"monitorMode": "future",
		"path":        "/media/adult/renamed",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body: %s", w.Code, w.Body.String())
	}
	var patched struct {
		Name        string `json:"name"`
		SortName    string `json:"sortName"`
		Status      string `json:"status"`
		MonitorMode string `json:"monitorMode"`
		Path        string `json:"path"`
	}
	decodeJSON(t, w, &patched)
	if patched.Name != "Renamed" {
		t.Errorf("name = %q, want Renamed", patched.Name)
	}
	if patched.SortName != "Renamed Sort" {
		t.Errorf("sortName = %q, want Renamed Sort", patched.SortName)
	}
	if patched.Status != "ended" {
		t.Errorf("status = %q, want ended", patched.Status)
	}
	if patched.MonitorMode != "future" {
		t.Errorf("monitorMode = %q, want future", patched.MonitorMode)
	}
	if patched.Path != "/media/adult/renamed" {
		t.Errorf("path = %q, want /media/adult/renamed", patched.Path)
	}
}

func TestGroups_Patch_AllFields(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "Show",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)

	w = do(t, h, http.MethodPost, "/api/v1/groups", map[string]any{
		"libraryEntryId": series.ID, "title": "S1", "number": 1,
	})
	var group struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &group)

	w = do(t, h, http.MethodPatch, "/api/v1/groups/"+group.ID, map[string]any{
		"sortName":    "Season One",
		"number":      10,
		"year":        2024,
		"overview":    "Great season",
		"monitorMode": "none",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch group status = %d, body: %s", w.Code, w.Body.String())
	}
	var patched struct {
		SortName    string `json:"sortName"`
		Number      int    `json:"number"`
		Year        int    `json:"year"`
		Overview    string `json:"overview"`
		MonitorMode string `json:"monitorMode"`
	}
	decodeJSON(t, w, &patched)
	if patched.Number != 10 {
		t.Errorf("number = %d, want 10", patched.Number)
	}
	if patched.Year != 2024 {
		t.Errorf("year = %d, want 2024", patched.Year)
	}
	if patched.MonitorMode != "none" {
		t.Errorf("monitorMode = %q, want none", patched.MonitorMode)
	}
}

func TestItems_Update_AllFields(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "tv", "kind": "series", "name": "Show",
	})
	var series struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &series)

	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": series.ID, "title": "Episode 1",
	})
	var item struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &item)

	w = do(t, h, http.MethodPatch, "/api/v1/items/"+item.ID, map[string]any{
		"title":          "Episode One",
		"overview":       "A great episode",
		"date":           "2024-03-15",
		"sequence":       "S01E01",
		"runtimeSeconds": 2700,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch item status = %d, body: %s", w.Code, w.Body.String())
	}
	var patched struct {
		Title          string `json:"title"`
		Overview       string `json:"overview"`
		Date           string `json:"date"`
		Sequence       string `json:"sequence"`
		RuntimeSeconds int    `json:"runtimeSeconds"`
	}
	decodeJSON(t, w, &patched)
	if patched.Title != "Episode One" {
		t.Errorf("title = %q, want Episode One", patched.Title)
	}
	if patched.Date != "2024-03-15" {
		t.Errorf("date = %q, want 2024-03-15", patched.Date)
	}
	if patched.Sequence != "S01E01" {
		t.Errorf("sequence = %q, want S01E01", patched.Sequence)
	}
	if patched.RuntimeSeconds != 2700 {
		t.Errorf("runtimeSeconds = %d, want 2700", patched.RuntimeSeconds)
	}
}

// ── Not-found for update / delete ─────────────────────────────────────────────

func TestGroups_Update_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPatch, "/api/v1/groups/no-such-id", map[string]any{"title": "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestItems_Update_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPatch, "/api/v1/items/no-such-id", map[string]any{"title": "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestPeople_Update_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPatch, "/api/v1/people/no-such-id", map[string]any{"name": "x"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestGroups_Delete_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodDelete, "/api/v1/groups/no-such-id", nil)
	// Delete of non-existent is idempotent — service returns no error for missing rows
	if w.Code != http.StatusNoContent && w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 204 or 404", w.Code)
	}
}

// ── Items list filters ─────────────────────────────────────────────────────────

func TestItems_List_ByStatus(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene Wanted",
	})
	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene Skipped",
	})
	var skipped struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &skipped)
	do(t, h, http.MethodPatch, "/api/v1/items/"+skipped.ID, map[string]any{"status": "skipped"})

	w = do(t, h, http.MethodGet, "/api/v1/items?status=wanted&libraryEntryId="+studio.ID, nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("wanted total = %d, want 1", resp.Total)
	}
}

func TestItems_List_BySearch(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	do(t, h, http.MethodPost, "/api/v1/items", map[string]any{"libraryEntryId": studio.ID, "title": "Hot Summer"})
	do(t, h, http.MethodPost, "/api/v1/items", map[string]any{"libraryEntryId": studio.ID, "title": "Winter Scene"})

	w = do(t, h, http.MethodGet, "/api/v1/items?search=Summer", nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("search Summer total = %d, want 1", resp.Total)
	}
}

func TestItems_List_Monitored_True(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	do(t, h, http.MethodPost, "/api/v1/items", map[string]any{"libraryEntryId": studio.ID, "title": "A", "monitored": true})
	do(t, h, http.MethodPost, "/api/v1/items", map[string]any{"libraryEntryId": studio.ID, "title": "B", "monitored": false})

	w = do(t, h, http.MethodGet, "/api/v1/items?monitored=true&libraryEntryId="+studio.ID, nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("monitored=true total = %d, want 1", resp.Total)
	}
}

func TestPeople_List_Monitored(t *testing.T) {
	h := newHandler(t)
	do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Alice", "monitored": true})
	do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Bob", "monitored": false})

	w := do(t, h, http.MethodGet, "/api/v1/people?monitored=true", nil)
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("monitored=true total = %d, want 1", resp.Total)
	}
}

func TestItems_Create_WithPeople(t *testing.T) {
	h := newHandler(t)

	// Create studio
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	// Create performer
	w = do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Jane Doe"})
	var performer struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &performer)

	// Create scene with performer
	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID,
		"title":          "Scene with People",
		"people":         []map[string]any{{"personId": performer.ID, "role": "performer"}},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create item with people: %d — %s", w.Code, w.Body.String())
	}
	var item struct {
		People []struct {
			PersonID string `json:"personId"`
			Role     string `json:"role"`
		} `json:"people"`
	}
	decodeJSON(t, w, &item)
	if len(item.People) != 1 {
		t.Errorf("people count = %d, want 1", len(item.People))
	}
	if item.People[0].Role != "performer" {
		t.Errorf("role = %q, want performer", item.People[0].Role)
	}
}

// ── Jobs ──────────────────────────────────────────────────────────────────────

func TestJobs_ListEmpty(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/api/v1/jobs", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var resp struct {
		Data  []any `json:"data"`
		Total int   `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Data == nil {
		t.Error("data should be [] not null")
	}
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
}

func TestJobs_Get_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodGet, "/api/v1/jobs/no-such-id", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", resp.Code)
	}
}

func TestJobs_Cancel_NotFound(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodDelete, "/api/v1/jobs/no-such-id", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestJobs_Cancel_SetsStatus(t *testing.T) {
	// Build a server backed by a queue we can inject a job into directly.
	dbPath := t.TempDir() + "/test.db"
	database, err := dbadapter.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	jobQueue := jobsadapter.New(1)
	t.Cleanup(jobQueue.Close)

	// Submit a blocking job so we can cancel it via the API.
	started := make(chan struct{})
	submitted, err := jobQueue.Submit(t.Context(), "blocker", nil, func(ctx context.Context, _ ports.ProgressReporter) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	<-started

	libSvc := library.New(
		dbadapter.NewLibraryEntryRepo(database),
		dbadapter.NewGroupRepo(database),
		dbadapter.NewItemRepo(database),
		dbadapter.NewPersonRepo(database),
	)
	peopleSvc := people.New(dbadapter.NewPersonRepo(database))
	metaSvc := metadata.New(nil, nil, dbadapter.NewLibraryEntryRepo(database), dbadapter.NewItemRepo(database), dbadapter.NewPersonRepo(database), dbadapter.NewTagRepo(database), dbadapter.NewExternalIDRepo(database), "")
	tagRepo := dbadapter.NewTagRepo(database)
	uiFS, _ := fs.Sub(web.Dist, "dist")
	cfg := &config.Config{
		Server:   config.ServerConfig{Port: 0, Workers: 1},
		Database: config.DatabaseConfig{Driver: "sqlite", DSN: dbPath},
		Log:      config.LogConfig{Level: "info", Format: "text"},
	}
	h := api.New(0, "", cfg, database, libSvc, peopleSvc, metaSvc, tagRepo, jobQueue, uiFS).Handler()

	// DELETE /api/v1/jobs/:id should cancel it.
	w := do(t, h, http.MethodDelete, "/api/v1/jobs/"+submitted.ID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("cancel status = %d, want 204", w.Code)
	}

	// Poll GET until terminal.
	deadline, ok := t.Deadline()
	if !ok {
		deadline = time.Now().Add(2 * time.Second)
	}
	for time.Now().Before(deadline) {
		w = do(t, h, http.MethodGet, "/api/v1/jobs/"+submitted.ID, nil)
		var job struct {
			Status string `json:"status"`
		}
		decodeJSON(t, w, &job)
		if job.Status == "cancelled" {
			return
		}
		if job.Status != "running" && job.Status != "queued" {
			t.Fatalf("unexpected status %q", job.Status)
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("job did not reach cancelled status within deadline")
}

// ── Commands ──────────────────────────────────────────────────────────────────

func TestCommands_RefreshStudio_Returns202(t *testing.T) {
	h := newHandler(t)

	// Create a studio entry so RefreshStudio has a valid target.
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "Test Studio",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create studio = %d", w.Code)
	}
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	w = do(t, h, http.MethodPost, "/api/v1/commands", map[string]any{
		"name":    "RefreshStudio",
		"entryId": studio.ID,
	})
	if w.Code != http.StatusAccepted {
		t.Fatalf("command status = %d, want 202 — body: %s", w.Code, w.Body.String())
	}
	var resp struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	decodeJSON(t, w, &resp)
	if resp.ID == "" {
		t.Error("job id should be set")
	}
	if resp.Name != "RefreshStudio" {
		t.Errorf("job name = %q, want RefreshStudio", resp.Name)
	}
}

func TestCommands_RefreshStudio_MissingEntryID(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/commands", map[string]any{
		"name": "RefreshStudio",
		// entryId omitted
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "MISSING_FIELDS" {
		t.Errorf("code = %q, want MISSING_FIELDS", resp.Code)
	}
}

// ── Artist entry people ───────────────────────────────────────────────────────

func TestLibraryEntries_Artist_GetIncludesPeople(t *testing.T) {
	h := newHandler(t)

	// Create artist entry
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "The Beatles",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create artist = %d, body: %s", w.Code, w.Body.String())
	}
	var artist struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &artist)

	// Create a person
	w = do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "John Lennon"})
	var person struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &person)

	// Link via DB directly (no API endpoint for this yet — tested at repo level)
	// GET should still return an empty people array (no members linked via API)
	w = do(t, h, http.MethodGet, "/api/v1/library-entries/"+artist.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get artist = %d", w.Code)
	}
	var resp struct {
		People []any `json:"people"`
	}
	decodeJSON(t, w, &resp)
	if resp.People == nil {
		t.Error("people should be [] not null")
	}
}

func TestLibraryEntries_List_FilterByPersonID(t *testing.T) {
	h := newHandler(t)

	// Create two artists
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "The Beatles",
	})
	var beatles struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &beatles)
	w = do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "Wings",
	})
	var wings struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &wings)

	// Create person
	w = do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Paul McCartney"})
	var paul struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &paul)

	// personId filter with no links → 0 results
	w = do(t, h, http.MethodGet, "/api/v1/library-entries?personId="+paul.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list by personId = %d", w.Code)
	}
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0 (no links yet)", resp.Total)
	}
}

func TestLibraryEntries_List_FilterByPersonID_WithLinks(t *testing.T) {
	h, db := newHandlerWithDB(t)

	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "The Beatles",
	})
	var beatles struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &beatles)
	do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "Wings",
	})

	w = do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "Paul McCartney"})
	var paul struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &paul)

	// Link Paul to The Beatles via DB (no API endpoint yet)
	if _, err := db.ExecContext(t.Context(),
		`INSERT INTO entry_people(library_entry_id, person_id, role, start_date, end_date) VALUES(?, ?, ?, ?, ?)`,
		beatles.ID, paul.ID, "member", "", "",
	); err != nil {
		t.Fatalf("insert entry_people: %v", err)
	}

	w = do(t, h, http.MethodGet, "/api/v1/library-entries?personId="+paul.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list by personId = %d", w.Code)
	}
	var resp struct {
		Total int `json:"total"`
	}
	decodeJSON(t, w, &resp)
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1 (paul is in beatles)", resp.Total)
	}
}

func TestLibraryEntries_Artist_PeopleInResponse(t *testing.T) {
	h, db := newHandlerWithDB(t)

	// Create artist entry
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "music", "kind": "artist", "name": "The Beatles",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create artist = %d, body: %s", w.Code, w.Body.String())
	}
	var artist struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &artist)

	// Create a person
	w = do(t, h, http.MethodPost, "/api/v1/people", map[string]any{"name": "John Lennon"})
	var person struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &person)

	// Link with start and end dates (no API endpoint yet — insert directly)
	if _, err := db.ExecContext(t.Context(),
		`INSERT INTO entry_people(library_entry_id, person_id, role, start_date, end_date) VALUES(?, ?, ?, ?, ?)`,
		artist.ID, person.ID, "member", "1960-01-01", "1970-12-31",
	); err != nil {
		t.Fatalf("insert entry_people: %v", err)
	}

	// GET artist — people array should be populated with person details and dates
	w = do(t, h, http.MethodGet, "/api/v1/library-entries/"+artist.ID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get artist = %d", w.Code)
	}
	var resp struct {
		People []struct {
			PersonID  string `json:"personId"`
			Role      string `json:"role"`
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
			Person    *struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"person"`
		} `json:"people"`
	}
	decodeJSON(t, w, &resp)

	if len(resp.People) != 1 {
		t.Fatalf("people count = %d, want 1", len(resp.People))
	}
	ep := resp.People[0]
	if ep.PersonID != person.ID {
		t.Errorf("personId = %q, want %q", ep.PersonID, person.ID)
	}
	if ep.Role != "member" {
		t.Errorf("role = %q, want member", ep.Role)
	}
	if ep.StartDate != "1960-01-01" {
		t.Errorf("startDate = %q, want 1960-01-01", ep.StartDate)
	}
	if ep.EndDate != "1970-12-31" {
		t.Errorf("endDate = %q, want 1970-12-31", ep.EndDate)
	}
	if ep.Person == nil {
		t.Fatal("person ref should be populated")
	}
	if ep.Person.Name != "John Lennon" {
		t.Errorf("person.name = %q, want John Lennon", ep.Person.Name)
	}
}

func TestCommands_UnknownCommand(t *testing.T) {
	h := newHandler(t)
	w := do(t, h, http.MethodPost, "/api/v1/commands", map[string]any{
		"name": "NonExistentCommand",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "UNKNOWN_COMMAND" {
		t.Errorf("code = %q, want UNKNOWN_COMMAND", resp.Code)
	}
}

// ── Item status transition enforcement ───────────────────────────────────────

func newItemInStudio(t *testing.T, h http.Handler) string {
	t.Helper()
	w := do(t, h, http.MethodPost, "/api/v1/library-entries", map[string]any{
		"contentType": "adult", "kind": "studio", "name": "S",
	})
	var studio struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &studio)

	w = do(t, h, http.MethodPost, "/api/v1/items", map[string]any{
		"libraryEntryId": studio.ID, "title": "Scene",
	})
	var item struct {
		ID string `json:"id"`
	}
	decodeJSON(t, w, &item)
	return item.ID
}

func TestItems_Patch_Status_SystemStatusRejected(t *testing.T) {
	h := newHandler(t)
	id := newItemInStudio(t, h)

	for _, s := range []string{"grabbed", "downloading", "imported", "missing"} {
		w := do(t, h, http.MethodPatch, "/api/v1/items/"+id, map[string]any{"status": s})
		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status %q: got HTTP %d, want 422", s, w.Code)
		}
		var resp struct {
			Code string `json:"code"`
		}
		decodeJSON(t, w, &resp)
		if resp.Code != "INVALID_STATUS" {
			t.Errorf("status %q: code = %q, want INVALID_STATUS", s, resp.Code)
		}
	}
}

func TestItems_Patch_Status_IllegalTransition(t *testing.T) {
	// Start from skipped, try to set to skipped (no-op is fine) then back to wanted.
	// The one illegal case we test here: grabbed → wanted (grabbed is pipeline-locked).
	// We can't set grabbed directly via PATCH (it's rejected above), so we test
	// the transition table via domain tests. Here we verify the HTTP response code
	// for a valid transition roundtrip.
	h := newHandler(t)
	id := newItemInStudio(t, h)

	// wanted → skipped (legal)
	w := do(t, h, http.MethodPatch, "/api/v1/items/"+id, map[string]any{"status": "skipped"})
	if w.Code != http.StatusOK {
		t.Fatalf("wanted→skipped: status = %d, body: %s", w.Code, w.Body.String())
	}

	// skipped → wanted (legal)
	w = do(t, h, http.MethodPatch, "/api/v1/items/"+id, map[string]any{"status": "wanted"})
	if w.Code != http.StatusOK {
		t.Fatalf("skipped→wanted: status = %d, body: %s", w.Code, w.Body.String())
	}

	var item struct {
		Status string `json:"status"`
	}
	decodeJSON(t, w, &item)
	if item.Status != "wanted" {
		t.Errorf("status = %q, want wanted", item.Status)
	}
}

func TestItems_Patch_Status_ImportedCannotBeSkipped(t *testing.T) {
	// We cannot set imported via PATCH (it's system-only), so we can only test
	// the domain layer directly for that transition. This test confirms that
	// the API rejects the imported status value itself.
	h := newHandler(t)
	id := newItemInStudio(t, h)

	w := do(t, h, http.MethodPatch, "/api/v1/items/"+id, map[string]any{"status": "imported"})
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	var resp struct {
		Code string `json:"code"`
	}
	decodeJSON(t, w, &resp)
	if resp.Code != "INVALID_STATUS" {
		t.Errorf("code = %q, want INVALID_STATUS", resp.Code)
	}
}
