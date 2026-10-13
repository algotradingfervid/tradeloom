// Package unix is a stand-in for golang.org/x/sys/unix (the fixture loads offline): the two
// process-starting functions g0-exec bans (Ruling 83) and one harmless call.
package unix

func Exec(argv0 string, argv []string, envv []string) error { return nil }

func ForkExec(argv0 string, argv []string, attr any) (int, error) { return 0, nil }

func Getpid() int { return 0 }

// Attr has a field named like an allowed function; Proc a method. g0-sys must match the allowed
// package-level objects by identity, not by "path.Name" (Ruling 99).
type Attr struct{ Exec bool }

type Proc struct{}

func (Proc) Exec() error { return nil }
