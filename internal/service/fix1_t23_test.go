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

// A user-chosen backup folder whose name holds glob metacharacters is swept literally: its own stale temp
// goes, and a sibling folder that the name would match as a pattern is untouched.
func TestBackupNow_SweepsLiteralFolderName(t *testing.T) {
	e := newSvcEnv(t)
	withData(t, e)
	ch := startEvents(t, e)
	root := t.TempDir()
	odd := filepath.Join(root, "Backup[1]")
	sibling := filepath.Join(root, "Backup1")
	for _, d := range []string{odd, sibling} {
		if err := os.Mkdir(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	mine := filepath.Join(odd, ".backup-1.tar.zst.tmp")
	other := filepath.Join(sibling, ".backup-2.tar.zst.tmp")
	writeFile(t, mine)
	writeFile(t, other)
	id, err := e.svc.BackupNow(context.Background(), odd, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, last := waitJob(t, ch, id); last.Error != "" {
		t.Fatalf("%+v", last)
	}
	if exists(mine) {
		t.Fatal("stale temp in the chosen folder kept")
	}
	if !exists(other) {
		t.Fatal("temp in a sibling folder removed")
	}
}
