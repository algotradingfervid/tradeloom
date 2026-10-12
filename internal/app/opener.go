package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"tradeloom/internal/service"
)

// openerTimeout bounds the desktop opener process (open / xdg-open hand the path to the file manager and exit).
const openerTimeout = 10 * time.Second

const openStage = "diagnostics"

// openerCommand builds the desktop opener for path: "open" on macOS, "xdg-open" on Linux. No shell is
// involved and the path is one argument (always absolute, so it can never be read as an option).
func openerCommand(ctx context.Context, goos, path string) (*exec.Cmd, error) {
	switch goos {
	case "darwin":
		return exec.CommandContext(ctx, "open", path), nil
	case "linux":
		return exec.CommandContext(ctx, "xdg-open", path), nil
	}
	return nil, fmt.Errorf("opening files is not supported on %s", goos)
}

// execOpenPath runs the desktop opener and reports its failure (Ruling 82 I1: no silent success).
func execOpenPath(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), openerTimeout)
	defer cancel()
	cmd, err := openerCommand(ctx, goruntime.GOOS, path)
	if err != nil {
		return err
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s failed: %w: %s", cmd.Args[0], err, strings.TrimSpace(string(out)))
	}
	return nil
}

func refusedOpen(code, reason string) error {
	return &service.ErrRefused{Code: code, Stage: openStage, Reason: reason}
}

// openPath opens the logs folder or a file directly inside it in the desktop's file manager / viewer.
// The path must be absolute, already clean, exist, and be the logs directory or a regular (non-symlink)
// file under it; anything else is refused (Ruling 82 I1).
func (a *App) openPath(p string) error {
	logs, err := a.d.Service.OpenLogsFolder(a.context())
	if err != nil {
		return err
	}
	if p == "" || !filepath.IsAbs(p) || filepath.Clean(p) != p || logs == "" || !filepath.IsAbs(logs) {
		return refusedOpen("invalid_path", "Only the logs folder and files in it can be opened.")
	}
	logs = filepath.Clean(logs)
	fi, err := os.Lstat(p)
	if err != nil {
		return refusedOpen("not_found", "The file to open does not exist.")
	}
	switch {
	case p == logs:
		if !fi.IsDir() {
			return refusedOpen("invalid_path", "The logs folder is not a directory.")
		}
	case strings.HasPrefix(p, logs+string(filepath.Separator)):
		if !fi.Mode().IsRegular() {
			return refusedOpen("invalid_path", "Only regular files in the logs folder can be opened.")
		}
	default:
		return refusedOpen("invalid_path", "Only the logs folder and files in it can be opened.")
	}
	if err := a.runtime().OpenPath(p); err != nil {
		a.d.Log.Warn("opening a path failed", "err", err)
		return refusedOpen("open_failed", "The file manager could not be started: "+err.Error())
	}
	return nil
}

// openURL opens an https URL (the Kite login page) in the system browser; anything else is refused and logged.
func (a *App) openURL(u string) {
	if !strings.HasPrefix(u, "https://") {
		a.d.Log.Warn("refusing to open a non-https URL in the browser")
		return
	}
	a.runtime().OpenURL(u)
}

var errNoWindow = errors.New("the window is not running")
