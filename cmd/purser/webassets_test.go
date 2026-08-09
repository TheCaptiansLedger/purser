package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	webui "purser/web"
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

// TestNewWebUIHandler_RealEmbedHasIndexHTML exercises webui.Assets
// itself, not fakeAssets() — every other test in this file proves
// newWebUIHandler's routing logic against a synthetic FS that always has
// an index.html, which can never catch the one failure mode that
// actually matters here: web/dist/index.html is gitignored (only
// dist/.gitkeep is committed — see web/embed.go and web/.gitignore), so
// a checkout that never ran `npm run build` (or a CI job that forgot the
// stub, as .github/workflows/pr.yml's k6 job once did) embeds a dist/
// with no index.html at all. mountWebUI treats that as an unrecoverable
// packaging bug and panics at serve startup — this test turns that into
// a fast, local `go test` failure instead, catching it in the pre-commit
// hook (make test-ci) before it ever reaches CI.
func TestNewWebUIHandler_RealEmbedHasIndexHTML(t *testing.T) {
	if _, err := newWebUIHandler(webui.Assets); err != nil {
		t.Fatalf("newWebUIHandler(webui.Assets) = %v, want nil — web/dist/index.html is missing; "+
			"run `make build web` (or `npm run build` in web/) before testing", err)
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
