package main

import "testing"

func TestNewAfterDarkCmd_HasExpectedSubcommands(t *testing.T) {
	cmd := newAfterDarkCmd()
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["stashdb"] || !names["theporndb"] {
		t.Errorf("afterdark command has subcommands %v, want stashdb and theporndb", names)
	}
}
