// Package unix is a stand-in for golang.org/x/sys/unix (the fixture loads offline): the two
// process-starting functions g0-exec bans (Ruling 83) and one harmless call.
package unix

func Exec(argv0 string, argv []string, envv []string) error { return nil }

func ForkExec(argv0 string, argv []string, attr any) (int, error) { return 0, nil }

func Getpid() int { return 0 }
