package filehash_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"purser/pkg/filehash"
	"testing"
)

func writeTemp(t *testing.T, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("writing fixture %s: %v", path, err)
	}
	return path
}

func TestMD5(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "d41d8cd98f00b204e9800998ecf8427e"},
		{"abc", []byte("abc"), "900150983cd24fb0d6963f7d28e17f72"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, dir, tc.name, tc.data)
			got, err := filehash.MD5(path)
			if err != nil {
				t.Fatalf("MD5: %v", err)
			}
			if got != tc.want {
				t.Errorf("MD5(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestSHA1(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"abc", []byte("abc"), "a9993e364706816aba3e25717850c26c9cd0d89d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, dir, tc.name, tc.data)
			got, err := filehash.SHA1(path)
			if err != nil {
				t.Fatalf("SHA1: %v", err)
			}
			if got != tc.want {
				t.Errorf("SHA1(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestSHA512(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, "cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e"},
		{"abc", []byte("abc"), "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTemp(t, dir, tc.name, tc.data)
			got, err := filehash.SHA512(path)
			if err != nil {
				t.Fatalf("SHA512: %v", err)
			}
			if got != tc.want {
				t.Errorf("SHA512(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestDigestFunctions_NonexistentPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	for name, fn := range map[string]func(string) (string, error){
		"MD5":    filehash.MD5,
		"SHA1":   filehash.SHA1,
		"SHA512": filehash.SHA512,
		"OSHash": filehash.OSHash,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := fn(missing); err == nil {
				t.Fatalf("%s(%q): expected error, got nil", name, missing)
			}
		})
	}
}

func TestOSHash_FileTooSmall(t *testing.T) {
	dir := t.TempDir()
	path := writeTemp(t, dir, "tiny", bytes.Repeat([]byte{0x01}, 65535))

	_, err := filehash.OSHash(path)
	if !errors.Is(err, filehash.ErrFileTooSmall) {
		t.Fatalf("OSHash: got err %v, want ErrFileTooSmall", err)
	}
}

func TestOSHash_ZeroFilledTwoChunks(t *testing.T) {
	const chunkSize = 64 * 1024
	dir := t.TempDir()
	// Exactly two non-overlapping chunks of zero bytes: every summed word
	// is 0, so the expected hash is trivially the file size itself.
	path := writeTemp(t, dir, "zeros", make([]byte, 2*chunkSize))

	got, err := filehash.OSHash(path)
	if err != nil {
		t.Fatalf("OSHash: %v", err)
	}
	want := fmt.Sprintf("%016x", uint64(2*chunkSize))
	if got != want {
		t.Errorf("OSHash = %q, want %q", got, want)
	}
}

func TestOSHash_WraparoundExactChunkSize(t *testing.T) {
	const chunkSize = 64 * 1024
	dir := t.TempDir()
	// A file exactly chunkSize bytes long makes head and tail read the
	// identical byte range (offset 0), so both chunks contribute the same
	// sum. Filling it with 0xFF makes every 8-byte word equal
	// ^uint64(0) (i.e. -1 mod 2^64), which forces the running sum through
	// several 64-bit wraps — proving sumWords wraps instead of panicking
	// or saturating. The expected total is computed here via uint64
	// multiplication (mod-2^64 by construction), an independent
	// arithmetic path from the implementation's loop of additions.
	path := writeTemp(t, dir, "ff", bytes.Repeat([]byte{0xFF}, chunkSize))

	got, err := filehash.OSHash(path)
	if err != nil {
		t.Fatalf("OSHash: %v", err)
	}

	words := uint64(chunkSize / 8)
	chunkSum := words * ^uint64(0) // words * 0xFFFFFFFFFFFFFFFF, wraps mod 2^64
	want := fmt.Sprintf("%016x", uint64(chunkSize)+chunkSum+chunkSum)
	if got != want {
		t.Errorf("OSHash = %q, want %q", got, want)
	}
}
