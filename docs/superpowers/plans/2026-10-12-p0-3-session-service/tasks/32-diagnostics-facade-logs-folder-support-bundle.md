> **P0-3 Session & Service Layer Implementation Plan — Task 32.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 32: Diagnostics façade — logs folder, support bundle, crash reports (FR-SHELL-018)

**Files:**
- Create: `internal/service/diagnostics.go`
- Modify: `internal/service/deps.go` (Task 14: one `Deps` field, the crash list on `Service`)
- Test: `internal/service/diagnostics_test.go`

**Interfaces:**
- Consumes: P0-1 `logging.BundleFile`, `logging.BundleOptions`, `logging.CreateSupportBundle`, `logging.MarkCrashSeen`, `logging.Default` (the process redactor), `(*DB).SupportFiles`, `store.Layout` (`LogsDir`, `ExportsDir`), `Boot.CrashReports`; Task 30 `beginJob`, `progress`, `JobSupportBundle`; Task 31 `calendarInfo`.
- Produces:
  ```go
  type CrashInfo struct{ Path string `json:"path"`; AtMs int64 `json:"atMs"` } // TS service.CrashInfo; Path "" = none
  // Deps gains: CrashReports []string (Boot.CrashReports: unseen crash-*.json, oldest first)
  func (s *Service) OpenLogsFolder(ctx context.Context) (string, error)      // Layout.LogsDir
  func (s *Service) CreateSupportBundle(ctx context.Context) (string, error) // zip path; job kind support_bundle with stages collecting|writing|done; bundle_failed, job_running
  func (s *Service) LastCrash(ctx context.Context) (CrashInfo, error)
  func (s *Service) OpenCrashReport(ctx context.Context) (string, error)     // the newest unseen report's path; no_crash
  func (s *Service) DismissCrash(ctx context.Context) error                  // marks the newest report seen; no_crash
  ```

The support bundle is P0-1's scrubbed zip (`logs/`, crash reports, `MANIFEST.txt`) plus the database's support files (`settings.json`, `versions.json`, `order_journal.json`, `deployment_logs.json`) and two files this plan adds: `calendar.json` (the `CalendarInfo` the UI shows) and `instruments.json` (`{"importedAtMs": …}`). It is written synchronously (the binding returns the path) but also reports as a job so the toast can show "Writing…" (A5). Crash reports come from P0-1's boot (`Boot.CrashReports`, the unseen `crash-*.json` files); `LastCrash` is the newest, `DismissCrash` renames it to `.seen.json` through `logging.MarkCrashSeen` so the next start does not show it again (FR-SHELL-018 AC3).

- [ ] **Step 1: Write the failing tests**

```go
// internal/service/diagnostics_test.go
package service

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tradeloom/internal/store"
	"tradeloom/internal/store/storetest"
)

func withDiagnostics(t *testing.T, e *svcEnv) store.Layout {
	t.Helper()
	layout := store.NewLayout(t.TempDir())
	if err := layout.Ensure(); err != nil {
		t.Fatal(err)
	}
	e.svc.d.Layout = layout
	e.svc.d.DB = storetest.Open(t)
	e.svc.d.Calendar = mustCalendar(t)
	e.svc.d.ImportedAt = func() time.Time { return ist(2026, 10, 12, 8, 30) }
	return layout
}

func zipNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	return names
}

// FR-SHELL-018 AC1/AC2: the logs folder and a support bundle with logs, crash reports and the data files.
func TestDiagnostics_LogsFolderAndSupportBundle(t *testing.T) {
	e := newSvcEnv(t)
	layout := withDiagnostics(t, e)
	ch := startEvents(t, e)
	ctx := context.Background()
	dir, err := e.svc.OpenLogsFolder(ctx)
	if err != nil || dir != layout.LogsDir {
		t.Fatalf("%q %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(layout.LogsDir, "tradeloom.log"), []byte("level=INFO msg=hello api_secret=TOPSECRET\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	crash := filepath.Join(layout.LogsDir, "crash-20261011-093000.json")
	if err := os.WriteFile(crash, []byte(`{"panic":"boom"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	e.svc.d.CrashReports = []string{crash}
	path, err := e.svc.CreateSupportBundle(ctx)
	if err != nil || filepath.Dir(path) != layout.ExportsDir || filepath.Ext(path) != ".zip" {
		t.Fatalf("%q %v", path, err)
	}
	stages, last := waitJob(t, ch, JobSupportBundle+"-1")
	if len(stages) != 3 || stages[0] != "collecting" || stages[1] != "writing" || stages[2] != "done" || last.ResultPath != path || last.Error != "" {
		t.Fatalf("%v %+v", stages, last)
	}
	names := zipNames(t, path)
	for _, want := range []string{"logs/tradeloom.log", "logs/crash-20261011-093000.json", "settings.json", "versions.json", "calendar.json", "instruments.json", "MANIFEST.txt"} {
		if !names[want] {
			t.Fatalf("bundle lacks %s: %v", want, names)
		}
	}
	rel := holdJob(t, e.svc)
	if _, err := e.svc.CreateSupportBundle(ctx); refusedCode(t, err) != "job_running" {
		t.Fatal(err)
	}
	rel()
	e.svc.d.Layout.ExportsDir = filepath.Join(layout.Root, "no", "such", "dir")
	if _, err := e.svc.CreateSupportBundle(ctx); refusedCode(t, err) != "bundle_failed" {
		t.Fatal(err)
	}
}

// FR-SHELL-018 AC3: the newest unseen crash report is offered once; dismissing marks it seen.
func TestDiagnostics_CrashReports(t *testing.T) {
	e := newSvcEnv(t)
	layout := withDiagnostics(t, e)
	ctx := context.Background()
	if info, err := e.svc.LastCrash(ctx); err != nil || info.Path != "" || info.AtMs != 0 {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := e.svc.OpenCrashReport(ctx); refusedCode(t, err) != "no_crash" {
		t.Fatal(err)
	}
	if err := e.svc.DismissCrash(ctx); refusedCode(t, err) != "no_crash" {
		t.Fatal(err)
	}
	older := filepath.Join(layout.LogsDir, "crash-20261010-120000.json")
	newer := filepath.Join(layout.LogsDir, "crash-20261011-093000.json")
	for _, p := range []string{older, newer} {
		if err := os.WriteFile(p, []byte(`{"panic":"boom"}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.svc.d.CrashReports = []string{older, newer}
	info, err := e.svc.LastCrash(ctx)
	if err != nil || info.Path != newer || info.AtMs == 0 {
		t.Fatalf("%+v %v", info, err)
	}
	if p, err := e.svc.OpenCrashReport(ctx); err != nil || p != newer {
		t.Fatalf("%q %v", p, err)
	}
	if err := e.svc.DismissCrash(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(newer); !os.IsNotExist(err) {
		t.Fatalf("report not renamed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(layout.LogsDir, "crash-20261011-093000.seen.json")); err != nil {
		t.Fatalf("seen file missing: %v", err)
	}
	if info, _ := e.svc.LastCrash(ctx); info.Path != older {
		t.Fatalf("after dismiss %+v", info)
	}
	if err := e.svc.DismissCrash(ctx); err != nil {
		t.Fatal(err)
	}
	if info, _ := e.svc.LastCrash(ctx); info.Path != "" {
		t.Fatalf("after second dismiss %+v", info)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -tags webkit2_41,noorders ./internal/service/ -run 'TestDiagnostics' -v`
Expected: FAIL to compile with `undefined: (*Service).OpenLogsFolder`, `e.svc.d.CrashReports undefined`.

- [ ] **Step 3: Extend `deps.go`**

Add to `Deps` (after `supportedSchema`):

```go
	CrashReports []string // Boot.CrashReports: unseen crash-*.json, oldest first (Task 32)
```

and to `Service` (after `jobSeq`):

```go
	crashMu sync.Mutex // guards d.CrashReports after New
```

- [ ] **Step 4: Write the implementation**

```go
// internal/service/diagnostics.go
package service

import (
	"context"
	"encoding/json"
	"os"

	"tradeloom/internal/logging"
)

const diagStage = "diagnostics"

// CrashInfo is the newest unseen crash report (Path "" when there is none).
type CrashInfo struct {
	Path string `json:"path"`
	AtMs int64  `json:"atMs"`
}

// OpenLogsFolder returns the logs directory; internal/app opens it in the file manager.
func (s *Service) OpenLogsFolder(_ context.Context) (string, error) {
	if s.d.Layout.LogsDir == "" {
		return "", refused(diagStage, "unavailable", "The logs folder is not configured.")
	}
	return s.d.Layout.LogsDir, nil
}

// CreateSupportBundle writes the scrubbed support zip into the exports folder (FR-SHELL-018 AC2).
func (s *Service) CreateSupportBundle(ctx context.Context) (string, error) {
	id, release, err := s.beginJob(JobSupportBundle)
	if err != nil {
		return "", err
	}
	defer release()
	step := func(stage string, n int) {
		s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: stage, Done: n, Total: 3})
	}
	fail := func(stage string, n int, err error) error {
		s.d.Log.Error("support bundle failed", "stage", stage, "err", err)
		s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: stage, Done: n, Total: 3, Finished: true, Error: err.Error()})
		return refused(diagStage, "bundle_failed", "The support bundle could not be written: "+err.Error())
	}
	step("collecting", 1)
	var files []logging.BundleFile
	if s.d.DB != nil {
		dbFiles, err := s.d.DB.SupportFiles(ctx)
		if err != nil {
			return "", fail("collecting", 1, err)
		}
		files = append(files, dbFiles...)
	}
	cal, err := json.MarshalIndent(s.calendarInfo(), "", "  ")
	if err != nil {
		return "", fail("collecting", 1, err)
	}
	files = append(files, logging.BundleFile{Name: "calendar.json", Data: cal})
	var importedAt int64
	if s.d.ImportedAt != nil {
		if at := s.d.ImportedAt(); !at.IsZero() {
			importedAt = at.UnixMilli()
		}
	}
	instr, err := json.Marshal(map[string]int64{"importedAtMs": importedAt})
	if err != nil {
		return "", fail("collecting", 1, err)
	}
	files = append(files, logging.BundleFile{Name: "instruments.json", Data: instr})
	step("writing", 2)
	path, err := logging.CreateSupportBundle(logging.BundleOptions{
		LogDir: s.d.Layout.LogsDir, ExportDir: s.d.Layout.ExportsDir, Files: files, Now: s.d.Clock.Now, Redactor: logging.Default,
	})
	if err != nil {
		return "", fail("writing", 2, err)
	}
	s.d.Log.Info("support bundle written", "path", path)
	s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: "done", Done: 3, Total: 3, Finished: true, ResultPath: path})
	return path, nil
}

// newestCrash returns the last entry of the unseen list that still exists on disk.
func (s *Service) newestCrash() (string, int64) {
	s.crashMu.Lock()
	defer s.crashMu.Unlock()
	for i := len(s.d.CrashReports) - 1; i >= 0; i-- {
		fi, err := os.Stat(s.d.CrashReports[i])
		if err != nil {
			s.d.CrashReports = append(s.d.CrashReports[:i], s.d.CrashReports[i+1:]...)
			continue
		}
		return s.d.CrashReports[i], fi.ModTime().UnixMilli()
	}
	return "", 0
}

func (s *Service) LastCrash(_ context.Context) (CrashInfo, error) {
	p, at := s.newestCrash()
	return CrashInfo{Path: p, AtMs: at}, nil
}

// OpenCrashReport returns the newest report's path; internal/app opens it.
func (s *Service) OpenCrashReport(_ context.Context) (string, error) {
	p, _ := s.newestCrash()
	if p == "" {
		return "", refused(diagStage, "no_crash", "There is no unseen crash report.")
	}
	return p, nil
}

// DismissCrash marks the newest report as seen (crash-*.json → crash-*.seen.json).
func (s *Service) DismissCrash(_ context.Context) error {
	p, _ := s.newestCrash()
	if p == "" {
		return refused(diagStage, "no_crash", "There is no unseen crash report.")
	}
	if err := logging.MarkCrashSeen(p); err != nil {
		return asRefused(diagStage, err)
	}
	s.crashMu.Lock()
	for i, c := range s.d.CrashReports {
		if c == p {
			s.d.CrashReports = append(s.d.CrashReports[:i], s.d.CrashReports[i+1:]...)
			break
		}
	}
	s.crashMu.Unlock()
	s.d.Log.Info("crash report dismissed", "path", p)
	return nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags webkit2_41,noorders -race ./internal/service/ -run 'TestDiagnostics' -v`
Expected: PASS (`TestDiagnostics_LogsFolderAndSupportBundle`, `TestDiagnostics_CrashReports`).

- [ ] **Step 6: Commit**

```bash
git add internal/service/diagnostics.go internal/service/diagnostics_test.go internal/service/deps.go
git commit -m "feat(service): logs folder, support bundle job and crash report access

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
