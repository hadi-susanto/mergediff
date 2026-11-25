//go:build !production

package main

import (
	"fmt"
	"os"
	"strings"
)

// devArgs supplies file arguments in development builds. `wails3 dev` starts
// the binary without arguments, so `make dev` passes sample files through
// MERGEDIFF_DEV_ARGS (space-separated). MERGEDIFF_DEV_DIR, when set, is
// the directory to run in (`make dev-resolve` points it at a conflicted
// sample repository). Production builds (-tags production) use
// devargs_production.go instead and never read the variables.
func devArgs(args []string) []string {
	if len(args) > 0 {
		return args
	}
	if dir := os.Getenv("MERGEDIFF_DEV_DIR"); dir != "" {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "mergediff: MERGEDIFF_DEV_DIR: %v\n", err)
		}
	}
	return strings.Fields(os.Getenv("MERGEDIFF_DEV_ARGS"))
}
