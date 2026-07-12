package externalid_test

import (
	"purser/internal/ports"
	"purser/internal/ports/externalidtest"
	"testing"

	memexternalid "purser/internal/adapters/memory/externalid"
)

func TestRepository_Contract(t *testing.T) {
	externalidtest.TestExternalIDRepository(t, func(t *testing.T) ports.ExternalIDRepository {
		t.Helper()
		r, err := memexternalid.New("contract-test")
		if err != nil {
			t.Fatalf("New returned error: %v", err)
		}
		return r
	})
}

func TestNew_ValidatesArguments(t *testing.T) {
	if _, err := memexternalid.New(""); err == nil {
		t.Fatal("New with empty name did not return an error")
	}
}
