// cmd/tradeloom/exit_test.go
package main

import "testing"

// A finished restore asks for a restart: run returns 3 and main re-execs the binary.
func TestRun_RestartExitCode(t *testing.T) {
	if got := finalExit(false); got != 0 {
		t.Fatalf("plain exit = %d", got)
	}
	if got := finalExit(true); got != exitRestart || exitRestart != 3 {
		t.Fatalf("restart exit = %d", got)
	}
}
