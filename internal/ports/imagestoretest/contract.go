// Package imagestoretest is the shared contract test suite for the
// ports.ImageStore port. See internal/ports/persontest for the
// convention this follows. Tests here stay black-box with respect to
// key: ports.ImageStore documents key as opaque, so nothing here asserts
// on its structure (e.g. a file extension suffix) — that would tie every
// implementation to the local-filesystem adapter's choices.
package imagestoretest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"purser/internal/ports"
	"testing"
)

// jpegMagic and pngMagic are real format signatures so
// http.DetectContentType-based sniffing (or any other implementation's
// sniffing) actually recognizes them as distinct image types.
var (
	jpegMagic = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pngMagic  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
)

// NewImageStoreFunc returns a fresh, empty ports.ImageStore for the
// duration of a single subtest.
type NewImageStoreFunc func(t *testing.T) ports.ImageStore

// TestImageStore runs the shared ImageStore contract against newStore.
func TestImageStore(t *testing.T, newStore NewImageStoreFunc) {
	t.Helper()

	t.Run("put then get round-trips the bytes", func(t *testing.T) { testPutThenGet(t, newStore) })
	t.Run("get with an unknown key returns ErrNotFound", func(t *testing.T) { testGetUnknownKey(t, newStore) })
	t.Run("delete with an unknown key returns ErrNotFound", func(t *testing.T) { testDeleteUnknownKey(t, newStore) })
	t.Run("delete then get returns ErrNotFound", func(t *testing.T) { testDeleteThenGet(t, newStore) })
	t.Run("put twice under the same owner/id overwrites", func(t *testing.T) { testPutOverwrites(t, newStore) })
	t.Run("different content types both round-trip", func(t *testing.T) { testDifferentContentTypes(t, newStore) })
	t.Run("multiple images under the same owner are independently retrievable", func(t *testing.T) { testMultipleImagesPerOwner(t, newStore) })
}

func mustPut(t *testing.T, s ports.ImageStore, ownerType, id string, data []byte) string {
	t.Helper()
	key, err := s.Put(context.Background(), ownerType, id, bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Put returned error: %v", err)
	}
	return key
}

func mustGet(t *testing.T, s ports.ImageStore, key string) []byte {
	t.Helper()
	rc, err := s.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("reading Get result returned error: %v", err)
	}
	return data
}

func testPutThenGet(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	key := mustPut(t, s, "person", "p1", jpegMagic)

	got := mustGet(t, s, key)
	if !bytes.Equal(got, jpegMagic) {
		t.Fatalf("Get returned %v, want %v", got, jpegMagic)
	}
}

func testGetUnknownKey(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	_, err := s.Get(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get with unknown key returned %v, want ErrNotFound", err)
	}
}

func testDeleteUnknownKey(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	err := s.Delete(context.Background(), "missing")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Delete with unknown key returned %v, want ErrNotFound", err)
	}
}

func testDeleteThenGet(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	key := mustPut(t, s, "person", "p1", jpegMagic)

	if err := s.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if _, err := s.Get(context.Background(), key); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Get after Delete returned %v, want ErrNotFound", err)
	}
}

func testPutOverwrites(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	first := mustPut(t, s, "person", "p1", jpegMagic)
	second := mustPut(t, s, "person", "p1", pngMagic)

	got := mustGet(t, s, second)
	if !bytes.Equal(got, pngMagic) {
		t.Fatalf("Get after overwrite returned %v, want %v", got, pngMagic)
	}
	if first != second {
		// Put for the same owner/id is documented as an overwrite, even
		// when a content-type change means the returned key itself
		// differs (e.g. a different file extension) — the old key must
		// no longer resolve, or the first blob is an orphan nothing ever
		// cleans up.
		if _, err := s.Get(context.Background(), first); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("Get(%q) after overwriting with a new key returned %v, want ErrNotFound (orphaned blob)", first, err)
		}
	}
}

func testDifferentContentTypes(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	jpegKey := mustPut(t, s, "person", "p1", jpegMagic)
	pngKey := mustPut(t, s, "person", "p2", pngMagic)

	if got := mustGet(t, s, jpegKey); !bytes.Equal(got, jpegMagic) {
		t.Fatalf("Get(jpegKey) returned %v, want %v", got, jpegMagic)
	}
	if got := mustGet(t, s, pngKey); !bytes.Equal(got, pngMagic) {
		t.Fatalf("Get(pngKey) returned %v, want %v", got, pngMagic)
	}
}

func testMultipleImagesPerOwner(t *testing.T, newStore NewImageStoreFunc) {
	s := newStore(t)
	key1 := mustPut(t, s, "person", "img1", jpegMagic)
	key2 := mustPut(t, s, "person", "img2", pngMagic)

	if key1 == key2 {
		t.Fatalf("Put for two different ids under the same owner returned the same key %q", key1)
	}
	if got := mustGet(t, s, key1); !bytes.Equal(got, jpegMagic) {
		t.Fatalf("Get(key1) returned %v, want %v", got, jpegMagic)
	}
	if got := mustGet(t, s, key2); !bytes.Equal(got, pngMagic) {
		t.Fatalf("Get(key2) returned %v, want %v", got, pngMagic)
	}
}
