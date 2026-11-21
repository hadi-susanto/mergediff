package main

import (
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"testing"
)

// The release workflow must run for vYYYY-MM-DD tags and nothing else.
func TestReleaseWorkflowTagFilter(t *testing.T) {
	data, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^\s+tags:\s*\["([^"]+)"\]\s*$`).FindSubmatch(data)
	if m == nil {
		t.Fatal(`no single-pattern tags: ["..."] filter found in release.yml`)
	}
	pattern := string(m[1])
	// GitHub's filter syntax matches path.Match for this pattern (no ** or +).
	for tag, want := range map[string]bool{
		"v2026-11-19":  true,
		"v2000-01-01":  true,
		"v1.0.0":       false,
		"v2026-11-19a": false,
		"2026-11-19":   false,
		"v2026-11-1":   false,
		"v26-11-19":    false,
	} {
		got, err := path.Match(pattern, tag)
		if err != nil {
			t.Fatalf("pattern %q: %v", pattern, err)
		}
		if got != want {
			t.Errorf("tag %q matches %q = %v, want %v", tag, pattern, got, want)
		}
	}
}

// scripts/ci/version-from-tag.sh validates the tag's date in every release job.
func TestVersionFromTag(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skip("bash not installed")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false",
		"commit", "-q", "--allow-empty", "-m", "x")
	script, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script += "/scripts/ci/version-from-tag.sh"

	for tag, ok := range map[string]bool{
		"v2026-11-19": true,
		"v2028-02-29": true, // leap year
		"v2000-02-29": true, // divisible by 400
		"v2026-12-31": true,
		"v2026-02-29": false,
		"v1900-02-29": false, // divisible by 100
		"v2026-04-31": false,
		"v2026-13-01": false,
		"v2026-00-10": false,
		"v2026-11-00": false,
		"v1.0.0":      false,
		"2026-11-19":  false,
	} {
		env := dir + "/env"
		os.Remove(env)
		cmd := exec.Command(bash, script, tag)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GITHUB_ENV="+env)
		out, err := cmd.CombinedOutput()
		if ok != (err == nil) {
			t.Errorf("%s: err = %v, want ok = %v\n%s", tag, err, ok, out)
			continue
		}
		if !ok {
			continue
		}
		written, err := os.ReadFile(env)
		if err != nil {
			t.Fatal(err)
		}
		date := strings.TrimPrefix(tag, "v")
		if !regexp.MustCompile(`^VERSION_DATE=` + date + `\nCOMMIT=[0-9a-f]{7}\n$`).Match(written) {
			t.Errorf("%s: GITHUB_ENV = %q", tag, written)
		}
	}
}
