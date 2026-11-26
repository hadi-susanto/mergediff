package main

import "testing"

func TestVersion(t *testing.T) {
	if got := Version(); got != "dev (unknown)" {
		t.Fatalf("Version() = %q without ldflags, want %q", got, "dev (unknown)")
	}
}
