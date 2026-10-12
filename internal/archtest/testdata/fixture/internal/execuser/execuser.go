// Package execuser starts processes outside internal/app. Violations: g0-exec (types) on every
// process-starting reference; g0-exec (imports) for syscall and golang.org/x/sys/unix; g0-net for os/exec.
package execuser

import (
	"os"
	x "os/exec"
	"syscall"

	"golang.org/x/sys/unix"

	"fixture/internal/app"
)

type cmd = x.Cmd // alias of os/exec.Cmd

func Run() {
	_ = x.Command("curl", "-X", "POST")
	_, _ = os.StartProcess("/usr/bin/curl", nil, &os.ProcAttr{})
	_ = syscall.Exec("/usr/bin/curl", nil, nil)
	_, _ = syscall.ForkExec("/usr/bin/curl", nil, nil)
	_, _, _ = syscall.StartProcess("/usr/bin/curl", nil, nil)
	_ = unix.Exec("/usr/bin/curl", nil, nil)
	_, _ = unix.ForkExec("/usr/bin/curl", nil, nil)
	_ = app.Opener("/tmp").Run() // os/exec method without naming os/exec
	start := os.StartProcess     // function value
	_ = start
	// Not process starts: no g0-exec (types) hit.
	_ = os.Getpid()
	_ = syscall.Getpid()
	_ = unix.Getpid()
	var _ *cmd
}
