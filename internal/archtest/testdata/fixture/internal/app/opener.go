// Package app is the one place os/exec may be used (g0-exec, Ruling 83): clean.
package app

import "os/exec"

// Opener hands a *exec.Cmd to its caller; a caller outside internal/app that uses it is flagged.
func Opener(path string) *exec.Cmd { return exec.Command("open", path) }
