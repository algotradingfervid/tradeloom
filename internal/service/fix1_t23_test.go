package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

// Ruling 89 (M4): a backup job first removes temp archives a crashed earlier backup left in its folder,
// and the quit-time (scheduled) backup ends with a terminal job.progress.
func TestScheduledBackupOnQuit_SweepsTempsAndReportsDone(t *testing.T) {
	folder := t.TempDir()
	stale := filepath.Join(folder, ".backup-123.tar.zst.tmp")
	other := filepath.Join(folder, "notes.tar.zst.tmp")
	writeFile(t, stale)
	writeFile(t, other)
	e, _, ch := scheduledEnv(t, folder)
	e.svc.BackupOnQuit(context.Background())
	jp := nextJobEvent(t, ch)
	if jp.Kind != JobBackup || !jp.Finished || jp.Error != "" || jp.Stage != "done" || jp.ResultPath == "" {
		t.Fatalf("%+v", jp)
	}
	if exists(stale) || !exists(other) {
		t.Fatalf("stale kept %v, unrelated removed %v", exists(stale), !exists(other))
	}
}

func TestBackupNow_SweepsStaleTemps(t *testing.T) {
	e := newSvcEnv(t)
	layout, _, _, _ := withData(t, e)
	ch := startEvents(t, e)
	stale := filepath.Join(layout.BackupsDir, ".backup-999.tar.zst.tmp")
	writeFile(t, stale)
	id, err := e.svc.BackupNow(context.Background(), "", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, last := waitJob(t, ch, id); last.Error != "" {
		t.Fatalf("%+v", last)
	}
	if exists(stale) {
		t.Fatal("stale temp archive kept")
	}
}
