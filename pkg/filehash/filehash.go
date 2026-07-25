// Package filehash computes file-identity hashes for the common scan
// pipeline (docs/adr/0024-pipeline-core.md). Every function is a pure,
// standalone algorithm over a single path — OSHash and SHA1 are always
// computed by callers, MD5/SHA512 are opt-in and gated by
// internal/config.Pipeline. This package exposes each algorithm
// independently; gating which ones run is entirely the caller's decision.
package filehash

import (
	"crypto/md5"  //nolint:gosec // G501: MD5 is used for file identification/dedup, not security
	"crypto/sha1" //nolint:gosec // G505: SHA-1 is used for file identification/dedup, not security
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
)

// chunkSize is the number of bytes OSHash reads from each end of a file —
// the fixed 64 KiB window the OpenSubtitles hash algorithm defines.
const chunkSize = 64 * 1024

// ErrFileTooSmall is returned by OSHash for files smaller than chunkSize
// bytes. The reference OpenSubtitles hash implementation handles small
// files by re-reading the same bytes from offset 0 for both the "head"
// and "tail" chunk — a quirky, effectively-undefined-for-non-multiple-of-8
// edge case this package deliberately does not replicate. Callers must
// handle this error per file rather than assume every discovered file
// yields an OSHash.
var ErrFileTooSmall = errors.New("filehash: file smaller than the minimum OSHash chunk size (65536 bytes)")

// OSHash computes the OpenSubtitles hash of the file at path: the file
// size plus the sum (as wrapping uint64 addition) of every 8-byte
// little-endian word in the file's first and last 64 KiB, formatted as 16
// lowercase hex characters. Cheap and partial-file by design — useful for
// identifying large video files without a full read. Returns
// ErrFileTooSmall for files under 64 KiB.
func OSHash(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is caller-supplied by design; hashing arbitrary discovered files is this package's purpose
	if err != nil {
		return "", fmt.Errorf("filehash: opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("filehash: stat %s: %w", path, err)
	}
	size := info.Size()
	if size < chunkSize {
		return "", ErrFileTooSmall
	}

	total := uint64(size)

	head := make([]byte, chunkSize)
	if _, err := io.ReadFull(f, head); err != nil {
		return "", fmt.Errorf("filehash: reading head of %s: %w", path, err)
	}
	total += sumWords(head)

	tail := make([]byte, chunkSize)
	if _, err := f.ReadAt(tail, size-chunkSize); err != nil {
		return "", fmt.Errorf("filehash: reading tail of %s: %w", path, err)
	}
	total += sumWords(tail)

	return fmt.Sprintf("%016x", total), nil
}

// sumWords adds every 8-byte little-endian uint64 word in buf, using
// ordinary Go uint64 addition — which wraps silently on overflow, giving
// the same modulo-2^64 result the reference C implementation gets from
// unsigned integer overflow. buf's length is always a multiple of 8
// (both OSHash callers pass exactly chunkSize bytes).
func sumWords(buf []byte) uint64 {
	var sum uint64
	for i := 0; i+8 <= len(buf); i += 8 {
		sum += binary.LittleEndian.Uint64(buf[i : i+8])
	}
	return sum
}

// MD5 computes the hex-encoded MD5 digest of the file at path.
func MD5(path string) (string, error) {
	return digest(path, md5.New()) //nolint:gosec // G401: MD5 used for file identification/dedup, not security
}

// SHA1 computes the hex-encoded SHA-1 digest of the file at path.
func SHA1(path string) (string, error) {
	return digest(path, sha1.New()) //nolint:gosec // G401: SHA-1 used for file identification/dedup, not security
}

// SHA512 computes the hex-encoded SHA-512 digest of the file at path.
func SHA512(path string) (string, error) {
	return digest(path, sha512.New())
}

// digest streams the file at path through h and returns its hex-encoded
// sum.
func digest(path string, h hash.Hash) (string, error) {
	f, err := os.Open(path) //nolint:gosec // path is caller-supplied by design; hashing arbitrary discovered files is this package's purpose
	if err != nil {
		return "", fmt.Errorf("filehash: opening %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("filehash: hashing %s: %w", path, err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
