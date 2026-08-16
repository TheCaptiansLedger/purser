package badger

import "strings"

const (
	schemaKey     = "\x00schema"
	primaryPrefix = "doc\x00"
	indexPrefix   = "idx\x00"
)

func kSchema() []byte { return []byte(schemaKey) }

// kPrimary is the key a document's envelope is stored under.
func kPrimary(collection, id string) []byte {
	return []byte(primaryPrefix + collection + "\x00" + id)
}

// pfxPrimary is the prefix every document key in collection shares, in ID
// sort order — a prefix scan yields IDs in ascending order for cursor
// pagination.
func pfxPrimary(collection string) []byte {
	return []byte(primaryPrefix + collection + "\x00")
}

// kIndex is the key a single secondary-index entry is stored under (empty
// value — existence is the signal).
func kIndex(collection, key, value, id string) []byte {
	return []byte(indexPrefix + collection + "\x00" + key + "\x00" + value + "\x00" + id)
}

// pfxIndex is the prefix every secondary-index entry for one filter
// key/value pair shares — a prefix scan yields every matching document ID.
func pfxIndex(collection, key, value string) []byte {
	return []byte(indexPrefix + collection + "\x00" + key + "\x00" + value + "\x00")
}

// DocumentPrefix is the key prefix every document lives under, across
// every collection — exported for internal/adapters/database/badger's raw
// keyspace walk (see docs/technical/database-backup-restore.md), which
// reads directly off the *badger.DB handle rather than going through
// Store, and so needs this package's key layout without duplicating it.
func DocumentPrefix() []byte { return []byte(primaryPrefix) }

// IndexPrefix is the key prefix every secondary-index entry lives under,
// across every collection — same use as DocumentPrefix, for Restore's
// clear step.
func IndexPrefix() []byte { return []byte(indexPrefix) }

// SplitDocumentKey recovers the collection and id a raw primary-document
// key (as produced by kPrimary, always DocumentPrefix()+collection+"\x00"+id)
// was stored under — the inverse operation, exported for the same raw
// keyspace walk as DocumentPrefix.
func SplitDocumentKey(key []byte) (collection, id string, ok bool) {
	s := string(key)
	rest, found := strings.CutPrefix(s, primaryPrefix)
	if !found {
		return "", "", false
	}
	parts := strings.SplitN(rest, "\x00", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
