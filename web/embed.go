package web

import "embed"

// Dist holds the embedded React build output served by the HTTP handler.
//
//go:embed all:dist
var Dist embed.FS
