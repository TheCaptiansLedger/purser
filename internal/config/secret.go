package config

import "reflect"

// secretFieldPaths is the set of dotted config keys (the same namespace
// ApplyOverrides works in) whose value is sensitive — API keys, passwords,
// DSNs — per docs/adr/0028-layered-settings.md's "Secret masking" section.
// It's derived once from a `secret:"true"` struct tag living alongside
// each sensitive field's own `mapstructure` tag, rather than a
// hand-maintained list living somewhere else in the package: a new
// sensitive field is self-documenting at its point of declaration, and
// can't be added without the tag being visible right next to it.
var secretFieldPaths = computeSecretFieldPaths(reflect.TypeOf(Config{}), "")

// computeSecretFieldPaths walks t's fields (t must be a struct type),
// recursing into nested structs and joining each level's `mapstructure`
// tag with ".", mirroring the dotted-key namespace Viper/ApplyOverrides
// already use. A field tagged `secret:"true"` contributes its own dotted
// path and is not recursed into further (every secret field in this
// codebase is a scalar string).
func computeSecretFieldPaths(t reflect.Type, prefix string) map[string]bool {
	paths := map[string]bool{}
	if t.Kind() != reflect.Struct {
		return paths
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("mapstructure")
		if tag == "" || tag == "-" {
			continue
		}

		key := tag
		if prefix != "" {
			key = prefix + "." + tag
		}

		if field.Tag.Get("secret") == "true" {
			paths[key] = true
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			for k := range computeSecretFieldPaths(field.Type, key) {
				paths[k] = true
			}
		}
	}

	return paths
}

// isSecretKey reports whether key — in ApplyOverrides's dotted namespace —
// is a sensitive field per secretFieldPaths.
func isSecretKey(key string) bool {
	return secretFieldPaths[key]
}
