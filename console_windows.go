//go:build windows

package main

import (
	"os"
	"syscall"
)

// Windows builds are GUI-subsystem programs (-H windowsgui), so they start
// without stdout/stderr unless the caller redirected them. Attach to the
// parent's console (cmd, PowerShell, Git Bash) so --help, --version and
// error messages are visible. Redirected handles are left alone.
func init() {
	if usable(os.Stdout) && usable(os.Stderr) {
		return
	}
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	attach := syscall.NewLazyDLL("kernel32.dll").NewProc("AttachConsole")
	if attach.Find() != nil {
		return
	}
	if ok, _, _ := attach.Call(attachParentProcess); ok == 0 {
		return // started without a console, e.g. from Explorer
	}
	con, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	if !usable(os.Stdout) {
		os.Stdout = con
	}
	if !usable(os.Stderr) {
		os.Stderr = con
	}
}

// usable reports whether f is backed by an open handle.
func usable(f *os.File) bool {
	if f == nil {
		return false
	}
	h := syscall.Handle(f.Fd())
	if h == 0 || h == syscall.InvalidHandle {
		return false
	}
	_, err := syscall.GetFileType(h)
	return err == nil
}
