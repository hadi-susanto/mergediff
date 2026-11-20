//go:build !production

package main

import (
	"os"
	"strings"
)

// devArgs supplies file arguments in development builds. `wails3 dev` starts
// the binary without arguments, so `make dev` passes sample files through
// MERGEDIFF_DEV_ARGS (space-separated). Production builds (-tags production)
// use devargs_production.go instead and never read the variable.
func devArgs(args []string) []string {
	if len(args) > 0 {
		return args
	}
	return strings.Fields(os.Getenv("MERGEDIFF_DEV_ARGS"))
}
