package main

import "testing"

func TestRunPrintsUsageWhenHelpFlagAppearsAnywhere(t *testing.T) {
	tests := [][]string{
		{"--help"},
		{"-h"},
		{"set", "--help"},
		{"get", "--help"},
		{"list", "--help"},
		{"set", "service", "key", "-h"},
	}

	for _, args := range tests {
		if code := run(args); code != 0 {
			t.Fatalf("run(%q) returned %d, want 0", args, code)
		}
	}
}

func TestHasHelpFlag(t *testing.T) {
	if !hasHelpFlag([]string{"set", "service", "--help"}) {
		t.Fatal("hasHelpFlag did not find --help")
	}
	if !hasHelpFlag([]string{"get", "-h"}) {
		t.Fatal("hasHelpFlag did not find -h")
	}
	if hasHelpFlag([]string{"set", "service", "key", "--", "--help"}) {
		t.Fatal("hasHelpFlag found help after -- separator")
	}
	if hasHelpFlag([]string{"get", "service", "key"}) {
		t.Fatal("hasHelpFlag found help in args without help flag")
	}
}
