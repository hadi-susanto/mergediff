package main

var (
	versionDate = "dev"     // set via -ldflags "-X main.versionDate=YYYY-MM-DD"
	commit      = "unknown" // set via -ldflags "-X main.commit=a1b2c3d"
)

// Version returns the build version, e.g. "2026-11-19 (a1b2c3d)".
func Version() string { return versionDate + " (" + commit + ")" }
