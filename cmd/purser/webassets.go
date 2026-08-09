package main

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	webui "purser/web"
)

// mountWebUI registers the embedded frontend build at "/" on mux —
// pulled out of newServeMux purely to keep its cyclomatic complexity
// under budget, same reasoning wireScanPipeline's own extraction comment
// gives, no behavior difference from being inlined there.
//
// Unlike every other construction step in newServeMux, a
// newWebUIHandler error here isn't a possible-at-runtime operational
// failure (bad config, an unreachable dependency) — dist/.gitkeep plus
// the embed directive's own zero-files build failure (web/embed.go)
// already guarantee dist/ and dist/index.html exist in any binary that
// compiled at all. An error here means the embed itself is broken, a
// packaging bug caught by TestNewWebUIHandler_* long before this ever
// runs — so it panics, same posture as an invariant-violation
// MustCompile, rather than threading one more `if err != nil` branch
// through newServeMux for a case that can't occur outside a broken
// build.
func mountWebUI(mux *http.ServeMux) {
	handler, err := newWebUIHandler(webui.Assets)
	if err != nil {
		panic(fmt.Sprintf("cmd/purser: web UI embed is broken (packaging bug, not a runtime condition): %v", err))
	}
	mux.Handle("/", handler)
}

// newWebUIHandler serves the embedded frontend build (assets, rooted at
// "dist" inside the embed.FS) with an SPA fallback: any path that isn't a
// real file under dist/ is served index.html instead, so client-side
// routes (e.g. React Router's /some/route) resolve correctly on a hard
// refresh or a direct link, rather than 404ing against the Go server that
// has never heard of them.
//
// Mounted at "/" in newServeMux. Go's stdlib http.ServeMux (1.22+)
// matches the more specific Connect service patterns already registered
// there (e.g. /purser.job.v1.JobService/) ahead of this catch-all, so
// registration order doesn't matter — see docs/design/frontend-stack.md.
func newWebUIHandler(assets fs.FS) (http.Handler, error) {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, err
	}

	// index.html's bytes are read once and served directly for the
	// fallback case, rather than re-routing the request back through
	// fileServer with a rewritten path — http.FileServer special-cases
	// any request whose path resolves to "index.html" by redirecting to
	// "./" (a well-known gotcha for SPA-fallback handlers built on top of
	// it), which would turn every client-side route into a redirect loop
	// instead of a 200.
	index, err := fs.ReadFile(dist, "index.html")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" {
			if f, err := dist.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(index)
	}), nil
}
