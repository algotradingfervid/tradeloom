// internal/app/reexec.go
package app

import "syscall"

// execFn is syscall.Exec; tests replace it.
var execFn = syscall.Exec

// ReExec replaces the process with exe (the relaunch after a finished restore, FR-DATA-005; exit code 3
// in cmd/tradeloom). It returns only on failure. It lives in internal/app, the one package allowed to
// spawn or replace processes (Ruling 79/89), so cmd/tradeloom imports no syscall.
func ReExec(exe string, args, env []string) error { return execFn(exe, args, env) }
