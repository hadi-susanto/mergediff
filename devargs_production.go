//go:build production

package main

// devArgs returns args unchanged in production builds (see devargs.go).
func devArgs(args []string) []string { return args }
