package app

import "syscall"

// ReExec is allowed: internal/app may use syscall.Exec (Task 23's app.ReExec, Ruling 89).
func ReExec(exe string, args, env []string) error { return syscall.Exec(exe, args, env) }

// Pid is not: Getpid is not on internal/app's syscall allow-list (g0-sys).
func Pid() int { return syscall.Getpid() }
