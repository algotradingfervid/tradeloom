package service

import "syscall"

// Raw system calls in an allow-listed importer: only O_NONBLOCK is allowed here, so both are flagged (g0-sys).
func Raw() {
	_, _, _ = syscall.Syscall(0, 0, 0, 0)
	_, _, _ = syscall.RawSyscall(0, 0, 0, 0)
}
