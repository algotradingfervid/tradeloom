package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"tradeloom/internal/bus"
	"tradeloom/internal/logging"
	"tradeloom/internal/secrets"
	"tradeloom/internal/settings"
	"tradeloom/internal/store"
)

// BootOptions configures Start.
type BootOptions struct {
	DataDir      string           // resolved by store.ResolveDataDir
	AppVersion   string           // Version
	PID          int              // os.Getpid()
	Now          func() time.Time // clock.SystemNow
	Stderr       io.Writer        // optional log mirror (development); nil disables
	ProbeKeyring func() error     // nil means secrets.ProbeKeyringWithin(secrets.ProbeTimeout)
}

// Boot holds everything started before the window opens (ARCHITECTURE §3.2 up to wails.Run).
type Boot struct {
	Layout          store.Layout
	Lock            *store.Lock
	Log             *logging.Logger
	DB              *store.DB
	Bus             *bus.Bus
	Settings        *settings.Settings
	Secrets         secrets.Detection
	PreviousExit    store.ExitState
	CrashReports    []string // unseen crash-*.json, newest first
	CrashedLastTime bool     // FR-SHELL-018 AC2: show "The app crashed last time — view report"
	levelSub        *bus.Sub[settings.Changed]
}

// Start runs the pre-window startup sequence: layout → lock → logging + crash reporter →
// app.db open/migrate → bus → settings (log level applied live) → secrets backend detection.
// Errors: store.ErrLocked (another instance), *store.SchemaTooNewError, store.ErrCorrupt.
func Start(ctx context.Context, o BootOptions) (*Boot, error) {
	if o.ProbeKeyring == nil {
		o.ProbeKeyring = func() error { return secrets.ProbeKeyringWithin(secrets.ProbeTimeout) }
	}
	b := &Boot{Layout: store.NewLayout(o.DataDir)}
	// A restore that crashed between its two renames leaves no root but a <root>.replaced-* directory;
	// put it back before Ensure would create an empty root. Only when the root is missing: with a root
	// present another instance may be running and its <root>.restoring-* staging must not be touched.
	var recovered string
	if _, err := os.Stat(b.Layout.Root); os.IsNotExist(err) {
		if recovered, err = store.RecoverInterruptedRestore(b.Layout.Root); err != nil {
			return nil, err
		}
	}
	if err := b.Layout.Ensure(); err != nil {
		return nil, err
	}
	lock, err := store.AcquireLock(b.Layout, o.PID)
	if err != nil {
		return nil, err
	}
	b.Lock, b.PreviousExit = lock, lock.PreviousExit()
	fail := func(err error) (*Boot, error) {
		// The app never ran: put the previous exit marker back so the next start does not report a crash.
		if rerr := b.Lock.Restore(b.PreviousExit); rerr != nil && b.Log != nil {
			b.Log.With("component", "app").Error("lock.restore_failed", "err", rerr)
		}
		b.close()
		return nil, err
	}
	// The root exists and we hold the lock: this only sweeps stale <root>.restoring-* staging.
	if _, err := store.RecoverInterruptedRestore(b.Layout.Root); err != nil {
		return fail(err)
	}
	if b.Log, err = logging.New(logging.Options{Dir: b.Layout.LogsDir, Stderr: o.Stderr}); err != nil {
		return fail(err)
	}
	logging.InstallCrashReporter(logging.CrashOptions{
		Dir: b.Layout.LogsDir, AppVersion: o.AppVersion, Now: o.Now,
		SchemaVersion: func() int {
			if b.DB == nil {
				return 0
			}
			return b.DB.SchemaVersion()
		},
	})
	if b.CrashReports, err = logging.UnseenCrashReports(b.Layout.LogsDir); err != nil {
		return fail(err)
	}
	b.CrashedLastTime = b.PreviousExit == store.ExitUnclean || len(b.CrashReports) > 0
	log := b.Log.With("component", "app")
	log.Info("app.starting", "version", o.AppVersion, "data_dir", o.DataDir, "previous_exit", string(b.PreviousExit))
	if recovered != "" {
		log.Warn("store.restore_recovered", "from", recovered, "data_dir", o.DataDir)
	}

	if b.DB, err = store.Open(ctx, b.Layout.AppDB, store.Options{AppVersion: o.AppVersion,
		BackupDir: b.Layout.BackupsDir, Now: o.Now, Log: b.Log.Logger}); err != nil {
		log.Error("store.open_failed", "err", err)
		return fail(err)
	}
	b.Bus = bus.New(b.Log.Logger)
	if b.Settings, err = settings.Load(ctx, b.DB.SettingsRepo(), b.Bus); err != nil {
		return fail(err)
	}
	st, lv := b.Settings, b.Log.Level // captured: close() nils the Boot fields
	applyLevel(st, lv)
	b.levelSub = bus.Subscribe(b.Bus, settings.Topic, bus.SubOpts[settings.Changed]{Name: "app.loglevel", Buffer: 16})
	sub := b.levelSub
	logging.Go("app.loglevel", func() {
		for ev := range sub.C() {
			if ev.Key == settings.LogLevel.Name {
				applyLevel(st, lv)
			}
		}
	})
	b.Secrets = secrets.Detect(b.Layout.SecretsFile, o.ProbeKeyring)
	log.Info("secrets.detected", "mode", string(b.Secrets.Mode), "keyring", b.Secrets.KeyringAvailable,
		"probe_error", errString(b.Secrets.ProbeError))
	return b, nil
}

func applyLevel(st *settings.Settings, lv *slog.LevelVar) {
	lvl, err := logging.ParseLevel(settings.Get(st, settings.LogLevel))
	if err != nil {
		lvl = slog.LevelInfo
	}
	lv.Set(lvl)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Shutdown is the clean-exit path (ARCHITECTURE §3.3 steps 5–7): bus close, app.db checkpoint +
// close, exit marker, lock release, log close. The exit is marked clean only when app.db closed
// cleanly; otherwise the next start reports the previous run as unclean. Safe to call more than once.
func (b *Boot) Shutdown() error {
	if b.Lock == nil { // already shut down or closed
		return nil
	}
	var log *slog.Logger
	if b.Log != nil {
		log = b.Log.With("component", "app")
		log.Info("app.shutdown")
	}
	if b.Bus != nil {
		b.Bus.Close()
		b.Bus = nil
	}
	var err error
	if b.DB != nil {
		if cerr := b.DB.Close(); cerr != nil {
			err = fmt.Errorf("app: close app.db: %w", cerr)
		}
		b.DB = nil
	}
	if err == nil {
		err = b.Lock.MarkClean()
	}
	if err != nil && log != nil {
		log.Error("app.shutdown_failed", "err", err)
	}
	b.close()
	return err
}

// close releases whatever Start acquired, without marking a clean exit. Idempotent.
func (b *Boot) close() {
	if b.Bus != nil {
		b.Bus.Close()
		b.Bus = nil
	}
	if b.DB != nil {
		b.DB.Close()
		b.DB = nil
	}
	if b.Lock != nil {
		b.Lock.Release()
		b.Lock = nil
	}
	if b.Log != nil {
		b.Log.Close()
		b.Log = nil
	}
}

// ExitMessage maps a Start error to the single line printed on stderr before exiting.
func ExitMessage(dataDir string, err error) string {
	var tooNew *store.SchemaTooNewError
	switch {
	case errors.Is(err, store.ErrLocked):
		return fmt.Sprintf("Tradeloom is already running (data dir %s)", dataDir)
	case errors.As(err, &tooNew):
		return tooNew.Error()
	default:
		return "Tradeloom could not start: " + err.Error()
	}
}
