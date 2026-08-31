package local

import "testing"

func TestExtFromContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/webp", ".webp"},
		{"image/gif", ".gif"},
		{"image/svg+xml", ".svg"},
		{"application/octet-stream", ".jpg"},
		{"", ".jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.ct, func(t *testing.T) {
			if got := extFromContentType(tt.ct); got != tt.want {
				t.Fatalf("extFromContentType(%q) = %q, want %q", tt.ct, got, tt.want)
			}
		})
	}
}

func TestShard(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"", ""},
		{"a", "a"},
		{"ab", "ab"},
		{"abcdef", "ab"},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := shard(tt.id); got != tt.want {
				t.Fatalf("shard(%q) = %q, want %q", tt.id, got, tt.want)
			}
		})
	}
}
