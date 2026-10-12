// internal/service/diagnostics.go
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
)

const diagStage = "diagnostics"

// Support-bundle bounds (variables so tests can lower them). Logs rotate at 20 MB × 10 files
// (FR-SHELL-018), so the defaults keep a complete log set; crash reports are small and capped harder.
var (
	bundleMaxFileBytes  int64 = 32 << 20 // per log file: only the newest whole lines are kept
	bundleMaxLogFiles         = 10       // newest tradeloom*.log files by modification time
	bundleMaxCrashFiles       = 20       // newest crash-*.json files by name (their timestamp)
)

// stageOpenHook runs before each log or crash report is opened for staging; tests use it to make a file
// vanish or change between the directory listing and the read.
var stageOpenHook = func(path string) {}

// bundleMaxCrashBytes is the per-report cap; a larger crash report is replaced by a note (a cut JSON
// document could split a secret the redactor would then not recognise).
const bundleMaxCrashBytes int64 = 1 << 20

// CrashInfo is the newest unseen crash report (Path "" when there is none).
type CrashInfo struct {
	Path string `json:"path"`
	AtMs int64  `json:"atMs"`
}

// OpenLogsFolder returns the logs directory; internal/app opens it in the file manager.
func (s *Service) OpenLogsFolder(_ context.Context) (string, error) {
	if s.d.Layout.LogsDir == "" {
		return "", s.refuse(diagStage, refused(diagStage, "unavailable", "The logs folder is not configured."))
	}
	return s.d.Layout.LogsDir, nil
}

// CreateSupportBundle writes the scrubbed support zip into the exports folder (FR-SHELL-018 AC2).
// The logs are first copied into a private staging folder inside the exports folder: bounded in
// size, scrubbed by the process redactor and with the client id masked; P0-1's CreateSupportBundle
// then zips that copy (scrubbing every entry again) through a temp file.
func (s *Service) CreateSupportBundle(ctx context.Context) (string, error) {
	id, release, err := s.beginJob(JobSupportBundle)
	if err != nil {
		return "", s.refuse(diagStage, err)
	}
	defer release()
	step := func(stage string, n int) {
		s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: stage, Done: n, Total: 3})
	}
	fail := func(stage string, n int, err error) error {
		s.d.Log.Error("support bundle failed", "stage", stage, "err", err)
		s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: stage, Done: n, Total: 3, Finished: true, Error: err.Error()})
		return s.refuse(diagStage, refused(diagStage, "bundle_failed", "The support bundle could not be written: "+err.Error()))
	}
	step("collecting", 1)
	exports := s.d.Layout.ExportsDir
	if fi, err := os.Stat(exports); exports == "" || !filepath.IsAbs(exports) || err != nil || !fi.IsDir() {
		return "", fail("collecting", 1, errors.New("the exports folder does not exist"))
	}
	s.sweepBundleLeftovers(exports)
	mask := s.bundleMasker()
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
	for i := range files {
		files[i].Data = mask(logging.Default.ScrubBytes(files[i].Data))
	}
	staging, err := os.MkdirTemp(exports, ".support-stage-*")
	if err != nil {
		return "", fail("collecting", 1, err)
	}
	defer func() {
		if err := os.RemoveAll(staging); err != nil {
			s.d.Log.Warn("support bundle staging not removed", "name", filepath.Base(staging), "err", err)
		}
	}()
	if err := s.stageLogs(ctx, staging, mask); err != nil {
		return "", fail("collecting", 1, err)
	}
	step("writing", 2)
	path, err := logging.CreateSupportBundle(logging.BundleOptions{
		LogDir: staging, ExportDir: exports, Files: files, Now: s.d.Clock.Now, Redactor: logging.Default,
	})
	if err != nil {
		return "", fail("writing", 2, err)
	}
	s.d.Log.Info("support bundle written", "name", filepath.Base(path))
	s.progress(JobProgress{JobID: id, Kind: JobSupportBundle, Stage: "done", Done: 3, Total: 3, Finished: true, ResultPath: path})
	return path, nil
}

// bundleMasker shortens the home directory (it carries the OS user name, Ruling 78 I1) to "~", raw and
// JSON-escaped, and masks the signed-in Kite client id ("AB1234" → "AB****") wherever they appear.
// Logs already mask the client id at the source; this is defence in depth for stray lines and app.db rows.
func (s *Service) bundleMasker() func([]byte) []byte {
	var pairs [][2][]byte
	if home, err := os.UserHomeDir(); err == nil {
		home = filepath.Clean(home)
		if filepath.IsAbs(home) && home != filepath.Dir(home) { // never "/" or a volume root
			pairs = append(pairs, [2][]byte{[]byte(home), []byte("~")})
			if esc, err := json.Marshal(home); err == nil && string(esc[1:len(esc)-1]) != home {
				pairs = append(pairs, [2][]byte{esc[1 : len(esc)-1], []byte("~")})
			}
		}
	}
	if id := s.d.Session.State().UserID; len(id) >= 4 {
		pairs = append(pairs, [2][]byte{[]byte(id), []byte(kite.MaskUserID(id))})
	}
	return func(b []byte) []byte {
		for _, p := range pairs {
			b = bytes.ReplaceAll(b, p[0], p[1])
		}
		return b
	}
}

// sweepBundleLeftovers removes staging folders and temp zips an interrupted run left in the exports
// folder (Ruling 78 M4). The single job slot guarantees no bundle is being written right now.
func (s *Service) sweepBundleLeftovers(exports string) {
	des, err := os.ReadDir(exports)
	if err != nil {
		return
	}
	for _, de := range des {
		n := de.Name()
		stage, _ := filepath.Match(".support-stage-*", n)
		tmp, _ := filepath.Match(".support-*.zip.tmp", n)
		if !stage && !tmp {
			continue
		}
		if err := os.RemoveAll(filepath.Join(exports, n)); err != nil {
			s.d.Log.Warn("support bundle leftover not removed", "name", n, "err", err)
		}
	}
}

type logCandidate struct {
	name string
	fi   os.FileInfo
}

// stageLogs copies the newest logs and crash reports from the logs folder into dir, bounded,
// scrubbed and masked. Only regular files are read; symlinks are never followed.
func (s *Service) stageLogs(ctx context.Context, dir string, mask func([]byte) []byte) error {
	src := s.d.Layout.LogsDir
	if src == "" {
		return nil
	}
	des, err := os.ReadDir(src)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var logs, crashes []logCandidate
	for _, de := range des {
		n := de.Name()
		isLog, _ := filepath.Match("tradeloom*.log", n)
		isCrash, _ := filepath.Match("crash-*.json", n)
		if !isLog && !isCrash {
			continue
		}
		fi, err := os.Lstat(filepath.Join(src, n))
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		if isLog {
			logs = append(logs, logCandidate{n, fi})
		} else {
			crashes = append(crashes, logCandidate{n, fi})
		}
	}
	sort.Slice(logs, func(i, j int) bool {
		if !logs[i].fi.ModTime().Equal(logs[j].fi.ModTime()) {
			return logs[i].fi.ModTime().After(logs[j].fi.ModTime())
		}
		return logs[i].name > logs[j].name
	})
	sort.Slice(crashes, func(i, j int) bool { return crashes[i].name > crashes[j].name })
	if len(logs) > bundleMaxLogFiles {
		logs = logs[:bundleMaxLogFiles]
	}
	if len(crashes) > bundleMaxCrashFiles {
		crashes = crashes[:bundleMaxCrashFiles]
	}
	copyOne := func(c logCandidate, data []byte) error {
		return os.WriteFile(filepath.Join(dir, c.name), mask(logging.Default.ScrubBytes(data)), 0o600)
	}
	for _, c := range logs {
		if err := ctx.Err(); err != nil {
			return err
		}
		stageOpenHook(filepath.Join(src, c.name))
		data, err := readLogTail(filepath.Join(src, c.name), c.fi, bundleMaxFileBytes)
		if skipped(err) {
			data, err = []byte(fmt.Sprintf("[%s was rotated or removed while the support bundle was written; skipped]\n", c.name)), nil
		}
		if err != nil {
			return err
		}
		if err := copyOne(c, data); err != nil {
			return err
		}
	}
	crashCap := min(bundleMaxFileBytes, bundleMaxCrashBytes)
	for _, c := range crashes {
		stageOpenHook(filepath.Join(src, c.name))
		data, err := readCrash(filepath.Join(src, c.name), c.fi, crashCap)
		if skipped(err) {
			data, err = []byte(`{"omitted":"crash report was removed or replaced while the support bundle was written"}`+"\n"), nil
		}
		if err != nil {
			return err
		}
		if err := copyOne(c, data); err != nil {
			return err
		}
	}
	return nil
}

// errFileChanged: the file at a listed name is no longer the regular file the listing saw (rotated,
// replaced or swapped for a symlink).
var errFileChanged = errors.New("file changed while the support bundle was being written")

// skipped reports whether a staging read failed because the file vanished or changed after the
// listing; such a file is skipped with a note instead of failing the bundle (Ruling 78 M2).
func skipped(err error) bool {
	return errors.Is(err, errFileChanged) || errors.Is(err, os.ErrNotExist)
}

// openRegular opens path and checks it is still the regular file seen by Lstat (no symlink swap).
func openRegular(path string, seen os.FileInfo) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || !os.SameFile(fi, seen) {
		f.Close()
		return nil, errFileChanged
	}
	return f, nil
}

// readCrash reads a whole crash report, or returns a note when the opened file is larger than limit
// (checked on the open file, Ruling 78 M3, and again on the bytes read in case it grew meanwhile).
func readCrash(path string, seen os.FileInfo, limit int64) ([]byte, error) {
	note := []byte(fmt.Sprintf("{\"omitted\":\"crash report larger than %d bytes\"}\n", limit))
	f, err := openRegular(path, seen)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fi.Size() > limit {
		return note, nil
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return note, nil
	}
	return b, nil
}

// readLogTail returns the newest whole lines of a log file, at most limit bytes. Cutting only at line
// boundaries keeps every secret whole, so the redactor still recognises it.
func readLogTail(path string, seen os.FileInfo, limit int64) ([]byte, error) {
	f, err := openRegular(path, seen)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := fi.Size()
	if size <= limit {
		b, err := io.ReadAll(io.LimitReader(f, limit))
		if err != nil {
			return nil, err
		}
		return completeLines(b), nil
	}
	// Read one byte before the window to tell whether it starts on a line boundary.
	if _, err := f.Seek(size-limit-1, io.SeekStart); err != nil {
		return nil, err
	}
	b := make([]byte, limit+1)
	n, err := io.ReadFull(f, b)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	b = b[:n]
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		b = b[i+1:]
	} else {
		b = nil
	}
	return completeLines(b), nil
}

// completeLines drops a trailing partial line (the active log may be mid-write).
func completeLines(b []byte) []byte {
	if len(b) == 0 || b[len(b)-1] == '\n' {
		return b
	}
	if i := bytes.LastIndexByte(b, '\n'); i >= 0 {
		return b[:i+1]
	}
	return nil
}

// newestCrashLocked returns the unseen report with the greatest file name (crash-<UTC timestamp>.json)
// and its modification time. Entries that vanished, were already seen or lie outside the logs folder
// are dropped, so OpenCrashReport never hands internal/app an arbitrary path. Caller holds crashMu.
func (s *Service) newestCrashLocked() (string, int64) {
	logs := s.d.Layout.LogsDir
	kept := s.d.CrashReports[:0:0]
	best, bestAt := "", int64(0)
	for _, p := range s.d.CrashReports {
		name := filepath.Base(p)
		ok, _ := filepath.Match("crash-*.json", name)
		if logs == "" || filepath.Dir(filepath.Clean(p)) != filepath.Clean(logs) || !ok || strings.HasSuffix(name, ".seen.json") {
			continue
		}
		fi, err := os.Lstat(p)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		kept = append(kept, p)
		if best == "" || name > filepath.Base(best) {
			best, bestAt = p, fi.ModTime().UnixMilli()
		}
	}
	s.d.CrashReports = kept
	return best, bestAt
}

// LastCrash returns the newest unseen crash report (FR-SHELL-018 AC2: "The app crashed last time").
func (s *Service) LastCrash(_ context.Context) (CrashInfo, error) {
	s.crashMu.Lock()
	defer s.crashMu.Unlock()
	p, at := s.newestCrashLocked()
	return CrashInfo{Path: p, AtMs: at}, nil
}

// OpenCrashReport returns the newest report's path; internal/app opens it.
func (s *Service) OpenCrashReport(_ context.Context) (string, error) {
	s.crashMu.Lock()
	defer s.crashMu.Unlock()
	p, _ := s.newestCrashLocked()
	if p == "" {
		return "", s.refuse(diagStage, refused(diagStage, "no_crash", "There is no unseen crash report."))
	}
	return p, nil
}

// DismissCrash marks the newest report as seen (crash-*.json → crash-*.seen.json).
func (s *Service) DismissCrash(_ context.Context) error {
	s.crashMu.Lock()
	defer s.crashMu.Unlock()
	p, _ := s.newestCrashLocked()
	if p == "" {
		return s.refuse(diagStage, refused(diagStage, "no_crash", "There is no unseen crash report."))
	}
	if err := logging.MarkCrashSeen(p); err != nil {
		return s.refuse(diagStage, err)
	}
	for i, c := range s.d.CrashReports {
		if c == p {
			s.d.CrashReports = append(s.d.CrashReports[:i], s.d.CrashReports[i+1:]...)
			break
		}
	}
	s.d.Log.Info("crash report dismissed", "name", filepath.Base(p))
	return nil
}
