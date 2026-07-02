package domain

import (
	"fmt"
	"strconv"
	"time"
)

// ItemSortKey returns the canonical sort key for an item, encoding date, sequence, and
// title in lexicographic-ascending chronological order. Adapters store this field and use
// it for pagination-safe ORDER BY; no sort logic lives inside any adapter.
//
// Sequences that are pure integers (track numbers, episode numbers) are zero-padded to
// ten digits so that "10" sorts after "9" rather than after "1".
func ItemSortKey(date time.Time, sequence, title string) string {
	if date.IsZero() {
		date = DefaultItemDate
	}
	seq := sequence
	if n, err := strconv.Atoi(sequence); err == nil {
		seq = fmt.Sprintf("%010d", n)
	}
	return date.UTC().Format("2006-01-02") + "|" + seq + "|" + title
}

// GroupSortKey returns the canonical sort key for a group: zero-padded number then title,
// so lexicographic order matches numeric order.
func GroupSortKey(number int, title string) string {
	return fmt.Sprintf("%010d|%s", number, title)
}

// NameSortKey returns the canonical sort key for a named entity (LibraryEntry, Person),
// preferring the explicit sort name over the display name.
func NameSortKey(sortName, name string) string {
	if sortName != "" {
		return sortName
	}
	return name
}

// TagSortKey returns the canonical sort key for a tag: key then value.
func TagSortKey(key, value string) string {
	return key + "|" + value
}
