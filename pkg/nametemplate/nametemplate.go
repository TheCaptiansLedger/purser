// Package nametemplate holds the text/template.FuncMap shared by every
// content type's Organizer naming template (docs/adr/0024-pipeline-core.md,
// docs/technical/pipeline-music-organizer.md). Registered identically
// wherever a naming template is parsed — internal/config's startup
// validation and the generic internal/service.Organizer's render step —
// so a template behaves the same at both points. Kept as its own pkg/
// package (no dependency on internal/config or internal/service) so
// neither has to import the other just to share this.
package nametemplate

import (
	"reflect"
	"text/template"
)

// Funcs returns the FuncMap every naming template is parsed with.
func Funcs() template.FuncMap {
	return template.FuncMap{
		"default": defaultFunc,
	}
}

// defaultFunc returns val if it is non-zero, else fallback. Zero-ness is
// checked via reflect.Value.IsZero, covering "", 0, nil, and any other
// type's zero value generically — template data values come from a
// heterogeneous map[string]any (curated builder fields alongside raw
// Item/MediaFile metadata), so no single concrete type can be assumed.
// Sprig-compatible argument order (fallback first) so
// `{{.Metadata.isrc | default "unknown"}}` pipes val as default's last
// arg, matching the convention template authors coming from Helm/Sprig
// already expect.
func defaultFunc(fallback, val any) any {
	if val == nil {
		return fallback
	}
	if reflect.ValueOf(val).IsZero() {
		return fallback
	}
	return val
}
