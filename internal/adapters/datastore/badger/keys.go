package badger

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
