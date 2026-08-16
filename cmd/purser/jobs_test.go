package main

import "testing"

func TestNewJobsCmd_AddrFlagDefault(t *testing.T) {
	cmd := newJobsCmd()

	flag := cmd.Flags().Lookup("addr")
	if flag == nil { //nolint:staticcheck // SA5011 false positive: t.Fatal below halts the test via runtime.Goexit, flag is never nil past this point
		t.Fatal("expected --addr flag to be registered")
	}
	if got, want := flag.DefValue, "http://localhost:7474"; got != want { //nolint:staticcheck // SA5011 false positive: t.Fatal above halts the test via runtime.Goexit, flag is never nil here
		t.Errorf("--addr default = %q, want %q", got, want)
	}
	if cmd.Use != "jobs" {
		t.Errorf("cmd.Use = %q, want %q", cmd.Use, "jobs")
	}
}
