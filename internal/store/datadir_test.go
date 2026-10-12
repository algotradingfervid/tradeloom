package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDataDir(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	cases := []struct {
		name    string
		env     map[string]string
		want    string
		wantErr bool
	}{
		{"default", nil, "/home/u/.local/share/tradeloom", false},
		{"xdg", map[string]string{"XDG_DATA_HOME": "/x"}, "/x/tradeloom", false},
		{"override wins", map[string]string{"XDG_DATA_HOME": "/x", EnvDataDir: "/d/t"}, "/d/t", false},
		{"relative override rejected", map[string]string{EnvDataDir: "rel"}, "", true},
		{"relative xdg ignored", map[string]string{"XDG_DATA_HOME": "rel"}, "/home/u/.local/share/tradeloom", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveDataDir(env(tc.env), "/home/u")
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
		})
	}
}

func TestLayoutEnsure(t *testing.T) {
	l := NewLayout(filepath.Join(t.TempDir(), "tl"))
	if err := l.Ensure(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{l.Root, l.CandlesDir, l.LogsDir, l.ExportsDir, l.BackupsDir, l.LibraryDir, l.InstrumentsDir} {
		st, err := os.Stat(d)
		if err != nil || !st.IsDir() || st.Mode().Perm() != 0o700 {
			t.Fatalf("%s: %v %v", d, st, err)
		}
	}
}

func TestLock_SingleHolderAndExitMarker(t *testing.T) {
	l := NewLayout(t.TempDir())
	k1, err := AcquireLock(l, 100)
	if err != nil {
		t.Fatal(err)
	}
	if k1.PreviousExit() != ExitFirstRun {
		t.Fatalf("first run: %s", k1.PreviousExit())
	}
	if _, err := AcquireLock(l, 101); !errors.Is(err, ErrLocked) {
		t.Fatalf("second holder: want ErrLocked, got %v", err)
	}
	// Simulated crash: release without MarkClean.
	if err := k1.Release(); err != nil {
		t.Fatal(err)
	}
	k2, err := AcquireLock(l, 102)
	if err != nil {
		t.Fatal(err)
	}
	if k2.PreviousExit() != ExitUnclean {
		t.Fatalf("after crash: %s", k2.PreviousExit())
	}
	if err := k2.MarkClean(); err != nil {
		t.Fatal(err)
	}
	k2.Release()
	k3, err := AcquireLock(l, 103)
	if err != nil {
		t.Fatal(err)
	}
	defer k3.Release()
	if k3.PreviousExit() != ExitClean {
		t.Fatalf("after clean exit: %s", k3.PreviousExit())
	}
}
