package settings

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Keys owned by P0-1. Later sub-plans add their keys in their own files of this package.
var (
	// LogLevel is Settings → Diagnostics "debug logging" (FR-SHELL-018): "info" | "debug".
	LogLevel = Key[string]{Name: "log.level", Default: "info", Validate: OneOf("info", "debug")}
	// BackupScheduleEnabled turns on the daily scheduled backup (FR-DATA-005; off by default).
	BackupScheduleEnabled = Key[bool]{Name: "backup.schedule.enabled", Default: false}
	// BackupScheduleFolder is the user-chosen folder for scheduled backups ("" until chosen).
	BackupScheduleFolder = Key[string]{Name: "backup.schedule.folder", Default: "", Validate: AbsPathOrEmpty}
	// BackupScheduleKeep is N, the number of scheduled backups kept (default 7).
	BackupScheduleKeep = Key[int]{Name: "backup.schedule.keep", Default: 7, Validate: IntRange(1, 365)}
	// BackupIncludeCandles is the "include the candle store" checkbox of Backup now.
	BackupIncludeCandles = Key[bool]{Name: "backup.include_candles", Default: false}
)

// OneOf accepts only the listed strings.
func OneOf(allowed ...string) func(string) error {
	return func(v string) error {
		for _, a := range allowed {
			if v == a {
				return nil
			}
		}
		return fmt.Errorf("%q is not one of %v", v, allowed)
	}
}

// IntRange accepts lo ≤ v ≤ hi.
func IntRange(lo, hi int) func(int) error {
	return func(v int) error {
		if v < lo || v > hi {
			return fmt.Errorf("%d is outside %d…%d", v, lo, hi)
		}
		return nil
	}
}

// AbsPathOrEmpty accepts "" or an absolute path.
func AbsPathOrEmpty(v string) error {
	if v != "" && !filepath.IsAbs(v) {
		return errors.New("must be an absolute path")
	}
	return nil
}
