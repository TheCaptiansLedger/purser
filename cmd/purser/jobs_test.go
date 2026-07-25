package main

import "testing"

func TestNewJobsCmd_AddrFlagDefault(t *testing.T) {
	cmd := newJobsCmd()

	flag := cmd.Flags().Lookup("addr")
	if flag == nil {
		t.Fatal("expected --addr flag to be registered")
	}
	if got, want := flag.DefValue, "http://localhost:7474"; got != want {
		t.Errorf("--addr default = %q, want %q", got, want)
	}
	if cmd.Use != "jobs" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "jobs")
	}
}
