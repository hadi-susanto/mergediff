package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// crashDir returns the directory crash logs are written to. It is a variable
// so tests can redirect it.
var crashDir = func() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mergediff"), nil
}

// reportCrash writes a panic or fatal error with its stack trace to stderr
// and to a crash log, because a GUI app may have no visible stderr. The
// caller exits afterwards.
func reportCrash(stderr io.Writer, what string, value any, stack []byte) {
	report := fmt.Sprintf("mergediff %s: %s: %v\n\n%s\n", Version(), what, value, stack)
	fmt.Fprint(stderr, report)
	path, err := writeCrashLog(report, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "mergediff: write crash log: %v\n", err)
		return
	}
	fmt.Fprintf(stderr, "mergediff: crash log written to %s\n", path)
}

func writeCrashLog(report string, now time.Time) (string, error) {
	dir, err := crashDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// No ':' in the name: it is not allowed in Windows file names.
	name := "crash-" + now.UTC().Format("20060102-150405.000") + ".log"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
