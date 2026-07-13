package version_test

import (
	"purser/internal/version"
	"testing"
)

func TestString_ReturnsVersionOnlyWhenUnstamped(t *testing.T) {
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = "dev", "none", "unknown"
	})
	version.Version, version.Commit, version.Date = "dev", "none", "unknown"

	if got, want := version.String(), "dev"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestString_IncludesCommitAndDateWhenStamped(t *testing.T) {
	t.Cleanup(func() {
		version.Version, version.Commit, version.Date = "dev", "none", "unknown"
	})
	version.Version, version.Commit, version.Date = "1.2.3", "abc1234", "2026-01-01T00:00:00Z"

	got := version.String()
	want := "1.2.3 (abc1234, 2026-01-01T00:00:00Z)"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
