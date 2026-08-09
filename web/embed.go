// Package webui embeds the built frontend (web/dist) into the Go binary,
// so cmd/purser serve can ship the UI without a separate web server or
// process — see docs/design/frontend-stack.md.
//
// This file lives in web/, sibling to dist/, because go:embed patterns
// cannot traverse "..": a file in cmd/purser could not embed a path at
// the repository's web/dist without this indirection. cmd/purser imports
// this package instead.
//
// dist/ is gitignored build output (see .gitignore) except for
// dist/.gitkeep, which exists purely so this embed directive has at
// least one file to match on a fresh checkout — go:embed fails the build
// otherwise. `all:` is required for that same reason: a dotfile is
// excluded by a bare "dist" pattern. `make build`/`make _build-web`
// (`npm run build`) overwrites it with the real bundle.
package webui

import "embed"

// Assets is the built frontend, rooted at "dist" — see the package doc
// comment above for why this file lives here and why dist/.gitkeep must
// always be present.
//
//go:embed all:dist
var Assets embed.FS
