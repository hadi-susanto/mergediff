//go:build !production

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDevArgs(t *testing.T) {
	t.Setenv("MERGEDIFF_DEV_ARGS", " a.txt  b.txt ")
	if got := devArgs(nil); !reflect.DeepEqual(got, []string{"a.txt", "b.txt"}) {
		t.Errorf("devArgs(nil) = %q", got)
	}
	if got := devArgs([]string{"x", "y"}); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Errorf("explicit args must win, got %q", got)
	}
	t.Setenv("MERGEDIFF_DEV_ARGS", "")
	if got := devArgs(nil); len(got) != 0 {
		t.Errorf("devArgs with empty env = %q", got)
	}
}

func TestDevArgsDir(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(t.TempDir()) // restored after the test
	t.Setenv("MERGEDIFF_DEV_ARGS", "resolve")
	t.Setenv("MERGEDIFF_DEV_DIR", dir)
	if got := devArgs(nil); !reflect.DeepEqual(got, []string{"resolve"}) {
		t.Errorf("devArgs = %q", got)
	}
	wd, _ := os.Getwd()
	if real, _ := filepath.EvalSymlinks(dir); wd != dir && wd != real {
		t.Errorf("working directory = %q, want %q", wd, dir)
	}
}
