package main

import (
	"fmt"

	"github.com/pterm/pterm"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// printLookupResult prints msg as pretty-printed JSON (via protojson, which
// respects the message's own field names/presence — never a bare
// fmt.Sprintf/%+v dump) when jsonOutput is set, otherwise as a styled
// pterm bullet list built from fields, per docs/adr/0009-cli-stack.md's
// "--json bypasses pterm" rule. fields is ordered label/value pairs;
// entries with an empty value are skipped in the styled path so an
// unpopulated optional field (e.g. an album with no CD-art) doesn't clutter
// the list, but always included in the JSON path since that's the full,
// faithful wire response.
func printLookupResult(msg proto.Message, jsonOutput bool, fields []labeledField) error {
	if jsonOutput {
		data, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshaling response as JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}
	return pterm.DefaultBulletList.WithItems(bulletItems(fields)).Render()
}

// labeledField is one label/value pair printLookupResult's styled path
// renders as a bullet list entry.
type labeledField struct {
	label string
	value string
}

// bulletItems converts fields to pterm.BulletListItems, dropping any entry
// with an empty value — an unpopulated optional field (e.g. an album with
// no CD-art) shouldn't clutter the styled list; --json (printLookupResult's
// other path) always includes every field regardless, since that's the
// full, faithful wire response. Split out from printLookupResult so this
// filtering logic is unit-testable without going through pterm's actual
// terminal rendering.
func bulletItems(fields []labeledField) []pterm.BulletListItem {
	items := make([]pterm.BulletListItem, 0, len(fields))
	for _, f := range fields {
		if f.value == "" {
			continue
		}
		items = append(items, pterm.BulletListItem{Text: fmt.Sprintf("%s: %s", f.label, f.value)})
	}
	return items
}
