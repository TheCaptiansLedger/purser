package main

import "testing"

func TestNewMusicCmd_HasExpectedSubcommands(t *testing.T) {
	cmd := newMusicCmd()
	names := map[string]bool{}
	for _, c := range cmd.Commands() {
		names[c.Name()] = true
	}
	if !names["theaudiodb"] || !names["fanart"] {
		t.Errorf("music command has subcommands %v, want theaudiodb and fanart", names)
	}
}
