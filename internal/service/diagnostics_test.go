// internal/service/diagnostics_test.go
package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tradeloom/internal/kite"
	"tradeloom/internal/logging"
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

// zipEntries returns every entry's decompressed bytes by name.
func zipEntries(t *testing.T, path string) map[string][]byte {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	out := map[string][]byte{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b
	}
	return out
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
	if _, err := os.Stat(e.svc.d.Layout.ExportsDir); !os.IsNotExist(err) {
		t.Fatalf("a missing exports folder must not be created: %v", err)
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

// Boot.CrashReports is newest first (logging.UnseenCrashReports); the façade must pick the newest
// whatever the order, and never offer a path outside the logs folder.
func TestDiagnostics_CrashReports_OrderIndependentAndInsideLogsDir(t *testing.T) {
	e := newSvcEnv(t)
	layout := withDiagnostics(t, e)
	ctx := context.Background()
	older := filepath.Join(layout.LogsDir, "crash-20261010T120000.000Z.json")
	newer := filepath.Join(layout.LogsDir, "crash-20261011T093000.000Z.json")
	outside := filepath.Join(t.TempDir(), "crash-20991231T000000.000Z.json")
	for _, p := range []string{older, newer, outside} {
		if err := os.WriteFile(p, []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	e.svc.d.CrashReports = []string{outside, newer, older}
	if info, err := e.svc.LastCrash(ctx); err != nil || info.Path != newer {
		t.Fatalf("%+v %v", info, err)
	}
	if err := e.svc.DismissCrash(ctx); err != nil {
		t.Fatal(err)
	}
	if p, err := e.svc.OpenCrashReport(ctx); err != nil || p != older {
		t.Fatalf("%q %v", p, err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("a report outside the logs folder was touched: %v", err)
	}
}

// Security: plant every secret type in the logs, the crash reports, the settings table and app.db,
// and the client id in all of them; nothing may appear in any entry or entry name of the bundle,
// and no staging or temp file may be left in the exports folder.
func TestDiagnostics_SupportBundle_NoSecretsAnywhere(t *testing.T) {
	e := newSvcEnv(t)
	layout := withDiagnostics(t, e)
	ctx := context.Background()

	const (
		apiSecret    = "kiteApiSecretZq81xw7v"                            // registered by the secrets store when loaded
		accessToken  = "accTokZq81xw7vMLP0q2"                             // registered by the secrets store when loaded
		requestToken = "reqTokZq81xw7vUNREG1"                             // never registered: removed by the key patterns
		telegram     = "1234567890:AAHtgTokenZq81xw7vabcdEFGHijklMNOPqrs" // registered (keyring value)
		passphrase   = "passPhraseZq81xw7v"                               // never registered: removed by the key patterns
		keyringValue = "keyringOnlyValueZq81xw7v"                         // any other keyring value: registered
		secretsFile  = "SECRETS-ENC-CONTENT-Zq81xw7v"
		clientID     = "ZX9876"
	)
	for _, v := range []string{apiSecret, accessToken, telegram, keyringValue} {
		logging.RegisterSecret(v)
	}
	e.session.setState(kite.SessionState{State: kite.StateReady, DataPlan: kite.PlanConnect, SessionID: "s1", UserID: clientID})

	logLines := strings.Join([]string{
		`{"level":"INFO","msg":"raw","v":"` + apiSecret + `","t":"` + accessToken + `","tg":"` + telegram + `","k":"` + keyringValue + `"}`,
		`{"level":"DEBUG","msg":"GET /x?request_token=` + requestToken + `&api_secret=` + apiSecret + `"}`,
		`{"level":"DEBUG","msg":"login","request_token":"` + requestToken + `","passphrase":"` + passphrase + `","user":"` + clientID + `"}`,
		`{"level":"DEBUG","msg":"telegram","url":"https://api.telegram.org/bot` + telegram + `/sendMessage"}`,
		`{"level":"DEBUG","msg":"hdr","Authorization":"token key:` + accessToken + `"}`,
		`passphrase=` + passphrase + ` client ` + clientID,
	}, "\n") + "\n"
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(layout.LogsDir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("tradeloom.log", logLines)
	write("tradeloom-2026-10-11T10-00-00.000.log", logLines)
	write("crash-20261011T093000.000Z.json", `{"panic":"`+accessToken+`","user":"`+clientID+`","request_token":"`+requestToken+`"}`)
	if err := os.WriteFile(layout.SecretsFile, []byte(secretsFile), 0o600); err != nil {
		t.Fatal(err)
	}
	e.svc.d.CrashReports = []string{filepath.Join(layout.LogsDir, "crash-20261011T093000.000Z.json")}

	repo := e.svc.d.DB.SettingsRepo()
	for k, v := range map[string]string{
		"kite.api_secret":       `"` + apiSecret + `"`,
		"kite.access_token":     `"` + accessToken + `"`,
		"telegram.bot_token":    `"` + telegram + `"`,
		"secrets.passphrase":    `"` + passphrase + `"`,
		"ui.note":               `"` + keyringValue + ` ` + clientID + `"`,
		"kite.user_" + clientID: `"x"`,
	} {
		if err := repo.Save(ctx, k, v); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.svc.d.DB.W.ExecContext(ctx,
		`INSERT INTO order_journal(at, tag, kind, source, params, kite_order_id, status, message, stage) VALUES (?,?,?,?,?,?,?,?,?)`,
		1, "t", "place", "manual", `{"access_token":"`+accessToken+`","client":"`+clientID+`"}`, "o1", "OPEN",
		"request_token="+requestToken+" "+telegram, "x"); err != nil {
		t.Fatal(err)
	}

	path, err := e.svc.CreateSupportBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, path)
	if len(entries) == 0 {
		t.Fatal("empty bundle")
	}
	planted := []string{apiSecret, accessToken, requestToken, telegram, passphrase, keyringValue, secretsFile, clientID}
	for name, data := range entries {
		if strings.Contains(strings.ToLower(name), "secrets.enc") || strings.Contains(strings.ToLower(name), "keyring") {
			t.Fatalf("bundle entry %q must not exist", name)
		}
		for _, s := range planted {
			if strings.Contains(name, s) || bytes.Contains(data, []byte(s)) {
				t.Fatalf("bundle entry %q contains planted value %q:\n%s", name, s, data)
			}
		}
	}
	for _, want := range []string{"logs/tradeloom.log", "logs/tradeloom-2026-10-11T10-00-00.000.log", "logs/crash-20261011T093000.000Z.json", "settings.json", "order_journal.json"} {
		if _, ok := entries[want]; !ok {
			t.Fatalf("bundle lacks %s", want)
		}
	}
	if !bytes.Contains(entries["logs/tradeloom.log"], []byte(kite.MaskUserID(clientID))) {
		t.Fatalf("client id not masked in the log:\n%s", entries["logs/tradeloom.log"])
	}
	des, err := os.ReadDir(layout.ExportsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(des) != 1 || des[0].Name() != filepath.Base(path) {
		var got []string
		for _, d := range des {
			got = append(got, d.Name())
		}
		t.Fatalf("exports folder holds more than the bundle: %v", got)
	}
	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("bundle mode %v %v", fi, err)
	}
}

// The bundle is bounded: a log larger than the per-file cap keeps only its newest whole lines,
// at most the newest bundleMaxLogFiles logs and bundleMaxCrashFiles crash reports are included,
// and symlinks in the logs folder are never followed.
func TestDiagnostics_SupportBundle_Bounded(t *testing.T) {
	e := newSvcEnv(t)
	layout := withDiagnostics(t, e)
	ctx := context.Background()
	prevFile, prevLogs, prevCrash := bundleMaxFileBytes, bundleMaxLogFiles, bundleMaxCrashFiles
	bundleMaxFileBytes, bundleMaxLogFiles, bundleMaxCrashFiles = 64, 2, 2
	t.Cleanup(func() { bundleMaxFileBytes, bundleMaxLogFiles, bundleMaxCrashFiles = prevFile, prevLogs, prevCrash })

	var big strings.Builder
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&big, "line-%02d access_token=abcdefghij\n", i)
	}
	if err := os.WriteFile(filepath.Join(layout.LogsDir, "tradeloom.log"), []byte(big.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	base := ist(2026, 10, 1, 10, 0)
	for i, n := range []string{"tradeloom-2026-10-01T10-00-00.000.log", "tradeloom-2026-10-02T10-00-00.000.log"} {
		p := filepath.Join(layout.LogsDir, n)
		if err := os.WriteFile(p, []byte("old\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, base.Add(time.Duration(i)*time.Hour), base.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range []string{"crash-20261001T000000.000Z.json", "crash-20261002T000000.000Z.json", "crash-20261003T000000.seen.json"} {
		if err := os.WriteFile(filepath.Join(layout.LogsDir, n), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("OUTSIDE-FILE-CONTENT"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(layout.LogsDir, "tradeloom-link.log")); err != nil {
		t.Fatal(err)
	}

	path, err := e.svc.CreateSupportBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, path)
	logs := []string{}
	crashes := []string{}
	for n, b := range entries {
		if bytes.Contains(b, []byte("OUTSIDE-FILE-CONTENT")) {
			t.Fatalf("symlink followed: %s", n)
		}
		switch {
		case strings.HasPrefix(n, "logs/tradeloom"):
			logs = append(logs, n)
		case strings.HasPrefix(n, "logs/crash-"):
			crashes = append(crashes, n)
		}
	}
	if len(logs) != 2 || entries["logs/tradeloom.log"] == nil || entries["logs/tradeloom-2026-10-02T10-00-00.000.log"] == nil {
		t.Fatalf("logs %v", logs)
	}
	if len(crashes) != 2 || entries["logs/crash-20261003T000000.seen.json"] == nil || entries["logs/crash-20261002T000000.000Z.json"] == nil {
		t.Fatalf("crashes %v", crashes)
	}
	tail := entries["logs/tradeloom.log"]
	// (The P0-1 redactor is not idempotent: a re-scrubbed "[redacted]" gains a "]", so lines are counted, not bytes: the 64-byte raw window holds exactly lines 18 and 19.)
	if bytes.Count(tail, []byte("line-")) != 2 || !bytes.HasPrefix(tail, []byte("line-18 access_token=[redacted]")) || !bytes.Contains(tail, []byte("\nline-19 access_token=[redacted]")) ||
		!bytes.HasSuffix(tail, []byte("\n")) || bytes.Contains(tail, []byte("abcdefghij")) {
		t.Fatalf("tail %q", tail)
	}
}
