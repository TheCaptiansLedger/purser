package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

// fakeAssets mirrors the real embed.FS's shape (files rooted under
// "dist/", matching web/embed.go's //go:embed all:dist) without needing
// a real frontend build — see docs/adr/0003-go-testing-standards.md.
func fakeAssets() fstest.MapFS {
	return fstest.MapFS{
		"dist/index.html":    {Data: []byte("<html>spa shell</html>")},
		"dist/assets/app.js": {Data: []byte("console.log('app')")},
		"dist/favicon.ico":   {Data: []byte("icon")},
	}
}

func TestNewWebUIHandler_ServesExistingFile(t *testing.T) {
	handler, err := newWebUIHandler(fakeAssets())
	if err != nil {
		t.Fatalf("newWebUIHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "console.log('app')" {
		t.Fatalf("body = %q, want the real asset content", got)
	}
}

func TestNewWebUIHandler_FallsBackToIndexForUnknownPath(t *testing.T) {
	handler, err := newWebUIHandler(fakeAssets())
	if err != nil {
		t.Fatalf("newWebUIHandler: %v", err)
	}

	// /some/client/route doesn't exist under dist/ — it's a React Router
	// client-side route, which must still resolve to the SPA shell rather
	// than 404, or a hard refresh on any non-root route would break.
	req := httptest.NewRequest(http.MethodGet, "/some/client/route", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "<html>spa shell</html>" {
		t.Fatalf("body = %q, want the index.html fallback", got)
	}
}

func TestNewWebUIHandler_ServesIndexAtRoot(t *testing.T) {
	handler, err := newWebUIHandler(fakeAssets())
	if err != nil {
		t.Fatalf("newWebUIHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "<html>spa shell</html>" {
		t.Fatalf("body = %q, want index.html", got)
	}
}
