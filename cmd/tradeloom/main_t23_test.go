package main

import (
	"bytes"
	"context"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/app"
	"tradeloom/internal/secrets"
	"tradeloom/internal/store"
)

// Ruling 89 (M6): outside Linux there is no D-Bus hand-off; a second launch prints the plain message.
func TestRun_SecondInstanceNonLinuxPlainMessage(t *testing.T) {
	prev := goos
	t.Cleanup(func() { goos = prev })
	goos = "darwin"
	dir := filepath.Join(t.TempDir(), "tradeloom")
	first, err := app.Start(context.Background(), app.BootOptions{DataDir: dir, AppVersion: "test", PID: 1,
		Now: func() time.Time { return time.Unix(0, 0) }, ProbeKeyring: func() error { return secrets.ErrUnavailable }})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Shutdown()
	var stderr bytes.Buffer
	env := func(k string) string {
		if k == store.EnvDataDir {
			return dir
		}
		return ""
	}
	if code := run(nil, env, &stderr); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if got, want := stderr.String(), "Tradeloom is already running (data dir "+dir+")\n"; got != want {
		t.Fatalf("stderr %q, want %q", got, want)
	}
}

// Ruling 89 (I1): cmd/tradeloom imports no syscall (the re-exec is app.ReExec).
func TestCmd_ImportsNoSyscall(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pkgs {
		for name, f := range p.Files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			for _, im := range f.Imports {
				if im.Path.Value == `"syscall"` || im.Path.Value == `"os/exec"` {
					t.Errorf("%s imports %s", name, im.Path.Value)
				}
			}
		}
	}
}
