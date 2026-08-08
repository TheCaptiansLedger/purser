package nametemplate_test

import (
	"bytes"
	"purser/pkg/nametemplate"
	"testing"
	"text/template"
)

func TestFuncs_Default(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want string
	}{
		{name: "non-empty string kept", data: map[string]any{"V": "isrc-123"}, want: "isrc-123"},
		{name: "empty string falls back", data: map[string]any{"V": ""}, want: "unknown"},
		{name: "zero int falls back", data: map[string]any{"V": 0}, want: "unknown"},
		{name: "non-zero int kept", data: map[string]any{"V": 5}, want: "5"},
		{name: "nil falls back", data: map[string]any{"V": nil}, want: "unknown"},
		{name: "missing key falls back", data: map[string]any{}, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpl, err := template.New("t").Funcs(nametemplate.Funcs()).Parse(`{{.V | default "unknown"}}`)
			if err != nil {
				t.Fatalf("Parse returned error: %v", err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, tt.data); err != nil {
				t.Fatalf("Execute returned error: %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("rendered = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFuncs_DefaultDirect(t *testing.T) {
	fn, ok := nametemplate.Funcs()["default"].(func(any, any) any)
	if !ok {
		t.Fatalf("Funcs()[\"default\"] is not a func(any, any) any")
	}
	if got := fn("fallback", "value"); got != "value" {
		t.Errorf("default(fallback, value) = %v, want %q", got, "value")
	}
	if got := fn("fallback", ""); got != "fallback" {
		t.Errorf("default(fallback, \"\") = %v, want %q", got, "fallback")
	}
}
