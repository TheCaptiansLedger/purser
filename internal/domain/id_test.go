package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewID(t *testing.T) {
	t.Run("returns a parseable UUIDv7", func(t *testing.T) {
		id := NewID()

		parsed, err := uuid.Parse(id)
		if err != nil {
			t.Fatalf("NewID() = %q, not a parseable UUID: %v", id, err)
		}
		if parsed.Version() != 7 {
			t.Fatalf("NewID() version = %d, want 7", parsed.Version())
		}
	})

	t.Run("returns a unique value on every call", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 1000 {
			id := NewID()
			if seen[id] {
				t.Fatalf("NewID() returned duplicate value %q", id)
			}
			seen[id] = true
		}
	})

	t.Run("successive IDs sort chronologically", func(t *testing.T) {
		a := NewID()
		b := NewID()
		if a >= b {
			t.Fatalf("NewID() not chronologically sortable: %q >= %q", a, b)
		}
	})
}
