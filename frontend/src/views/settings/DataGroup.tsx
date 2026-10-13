import { useCallback, useEffect, useRef, useState } from 'react';
import { timeMs, type BackupInfo, type DataStats } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { Toggle } from '../../components/Toggle';
import { componentCopy, dataCopy as copy } from '../../i18n/copy';
import { DASH, formatBytes, formatDateTimeLong } from '../../i18n/format';
import { startManualBackup, useBackupNoticesStore } from '../../store/backupNotices';
import { startClaimedJob } from '../../store/jobNotices';
import { useJobsStore } from '../../store/jobs';
import { reloadSettings, resumeSettingsWrites, suspendSettingsWrites, useSetting } from '../../store/settings';
import { pushToast } from '../../store/toasts';
import { Fact, NumberCommitField, SettingSection } from './SettingRow';

export const RESTORE_PHRASE = 'RESTORE';

/** Go's `service.StageRestart`: a started restore always ends here and the app quits and relaunches (Rulings 74, 75). */
const STAGE_RESTART = 'restart';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

const COUNT_KEYS = ['strategies', 'scanners', 'backtest_runs', 'deployments', 'watchlists', 'order_journal'] as const;

function RestoreSummary({ manifest }: { manifest: BackupInfo }) {
  const created = timeMs(manifest.createdAt);
  return (
    <>
      <p>{copy.restoreFrom(created > 0 ? formatDateTimeLong(created) : DASH, manifest.appVersion)}</p>
      <ul>
        {COUNT_KEYS.map((k) => <li key={k}>{copy.count(copy.counts[k], manifest.counts?.[k] ?? 0)}</li>)}
      </ul>
      <p>{manifest.includesCandles ? copy.withCandles : copy.withoutCandles}</p>
      <p>{copy.restoreWarning}</p>
      <p>{copy.restoreSecrets}</p>
    </>
  );
}

function RestoreDialog({ manifest, onClose }: { manifest: BackupInfo; onClose: () => void }) {
  const backend = useBackend();
  const [jobId, setJobId] = useState<string | null>(null);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const job = useJobsStore((s) => (jobId ? s.jobs[jobId] : undefined));

  // A finished restore that does not restart (Go never sends one; defensive) hands settings writes back.
  useEffect(() => {
    if (!job?.finished || job.stage === STAGE_RESTART) return;
    const reason = job.error || copy.restoreEnded;
    setJobId(null);
    setError(reason);
    resumeSettingsWrites();
    void reloadSettings(backend).catch(() => undefined);
    pushToast({ level: 'error', message: copy.restoreFailed(reason) });
  }, [job, backend]);

  const start = async () => {
    if (starting) return;
    setStarting(true);
    setError(null);
    suspendSettingsWrites(); // review focus #5: nothing may write over the database being restored
    try {
      setJobId(await startClaimedJob(() => backend.data.restore(manifest.path, RESTORE_PHRASE)));
    } catch (e) {
      resumeSettingsWrites();
      setError(parseBackendError(e).reason);
    } finally {
      setStarting(false);
    }
  };

  if (!manifest.ok) {
    return (
      <Dialog open title={copy.cannotRestore} size="sm" onClose={onClose} footer={<Button onClick={onClose}>{componentCopy.close}</Button>}>
        <p role="alert">{manifest.reason}</p>
      </Dialog>
    );
  }
  if (jobId) {
    // Every started restore ends with stage "restart": Go quits with exit code 3 and `main` starts the app again,
    // with the restored data or, when Error is set, with the previous data put back (A5, Rulings 74, 75).
    // Settings writes stay paused until then.
    const restarting = job?.finished === true && job.stage === STAGE_RESTART;
    return (
      <Dialog open title={copy.restoring} size="sm" onClose={onClose} dismissible={false}>
        {restarting && job.error ? (
          <>
            <p role="alert">{copy.restoreFailed(job.error)}</p>
            <p role="status">{copy.restartingAfterFailure}</p>
          </>
        ) : (
          <p role="status">{restarting ? copy.restarting : copy.restoreStage(job?.done ?? 0, job?.total ?? 0)}</p>
        )}
      </Dialog>
    );
  }
  return (
    <ConfirmDialog
      open
      title={copy.restoreTitle}
      message={<RestoreSummary manifest={manifest} />}
      confirmLabel={copy.restoreConfirm}
      tone="danger"
      typedConfirmation={RESTORE_PHRASE}
      busy={starting}
      error={error}
      onConfirm={() => void start()}
      onCancel={onClose}
    />
  );
}

/** Retention for scheduled backups; the range is checked here so the message is ours (Go: IntRange(1, 365)). */
function KeepField() {
  const [keep, setKeep] = useSetting<number>('backup.schedule.keep', 7);
  return (
    <NumberCommitField
      label={copy.keep}
      value={keep}
      onCommit={async (n) => (n < 1 || n > 365 ? copy.keepRange : ((await setKeep(n))?.reason ?? null))}
    />
  );
}

function useMounted() {
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  return mounted;
}

export function DataGroup() {
  const backend = useBackend();
  const mounted = useMounted();
  const [stats, setStats] = useState<DataStats | null>(null);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [clearing, setClearing] = useState(false);
  const [busy, setBusy] = useState<'clear' | 'instruments' | 'backup' | null>(null);
  const [includeCandles, setIncludeCandles] = useSetting<boolean>('backup.include_candles', false);
  // Any unfinished backup (manual or scheduled) counts, so a remount still knows one is running (Ruling 106).
  const backupProgress = useJobsStore((s) => Object.values(s.jobs).find((j) => j.kind === 'backup' && !j.finished));
  const backupTracked = useBackupNoticesStore((s) => s.tracked.length > 0);
  const backupRunning = backupProgress !== undefined || backupTracked || busy === 'backup';
  const backupStatus = !backupRunning ? '' : backupProgress ? copy.backupStage(backupProgress.done, backupProgress.total) : copy.backingUp;
  const [manifest, setManifest] = useState<BackupInfo | null>(null);
  const [scheduled, setScheduled] = useSetting<boolean>('backup.schedule.enabled', false);
  const [folder, setFolder] = useSetting<string>('backup.schedule.folder', '');

  const loadStats = useCallback(async () => {
    try {
      const next = await backend.data.stats();
      if (!mounted.current) return;
      setStats(next);
      setStatsError(null);
    } catch (e) {
      if (mounted.current) setStatsError(parseBackendError(e).reason);
    }
  }, [backend, mounted]);
  useEffect(() => {
    void loadStats();
  }, [loadStats]);

  const clear = async () => {
    setBusy('clear');
    try {
      await backend.data.clearCandles();
      pushToast({ level: 'success', message: copy.cleared });
      await loadStats();
    } catch (e) {
      fail(e);
    } finally {
      if (mounted.current) {
        setClearing(false);
        setBusy(null);
      }
    }
  };
  const refreshInstruments = async () => {
    if (busy === 'instruments') return;
    setBusy('instruments');
    try {
      await backend.data.refreshInstruments();
      pushToast({ level: 'success', message: copy.instrumentsRefreshed });
      await loadStats();
    } catch (e) {
      fail(e);
    } finally {
      if (mounted.current) setBusy(null);
    }
  };
  const backupNow = async () => {
    if (backupRunning) return;
    setBusy('backup');
    try {
      const target = await backend.data.chooseFolder();
      if (!target || !mounted.current) return;
      // The claim and the result toast are owned by store/backupNotices, never by this component (Ruling 106).
      await startManualBackup(backend, target, includeCandles);
    } catch (e) {
      fail(e);
    } finally {
      if (mounted.current) setBusy(null);
    }
  };
  const chooseScheduledFolder = async () => {
    try {
      const target = await backend.data.chooseFolder();
      if (!target) return;
      const refused = await setFolder(target);
      if (refused) pushToast({ level: 'error', message: refused.reason });
    } catch (e) {
      fail(e);
    }
  };
  const startRestore = async () => {
    try {
      const path = await backend.data.chooseBackupFile();
      if (!path) return;
      const info = await backend.data.inspectBackup(path);
      if (mounted.current) setManifest(info);
    } catch (e) {
      fail(e);
    }
  };
  const toggleSetting = async (write: Promise<{ reason: string } | null>) => {
    const refused = await write;
    if (refused) pushToast({ level: 'error', message: refused.reason });
  };

  return (
    <>
      <SettingSection title={copy.storage}>
        {statsError && <p className="field__error" role="alert">{statsError}</p>}
        <dl className="facts">
          <Fact label={copy.dataDir}>{stats?.dataDir ?? DASH}</Fact>
          <Fact label={copy.appDb}>{formatBytes(stats?.appDbBytes)}</Fact>
          <Fact label={copy.candles}>{stats ? copy.candleUsage(formatBytes(stats.candleBytes), formatBytes(stats.candleBudgetBytes)) : DASH}</Fact>
          <Fact label={copy.instruments}>{stats && stats.instrumentsImportedAtMs > 0 ? formatDateTimeLong(stats.instrumentsImportedAtMs) : DASH}</Fact>
          <Fact label={copy.backfill}>{stats ? copy.backfillOpen(stats.backfillOpenJobs) : DASH}</Fact>
        </dl>
        <div className="settings__actions">
          {/* aria-disabled, not disabled: focus stays on a busy button (Ruling 106). */}
          <Button onClick={() => void refreshInstruments()} aria-disabled={busy === 'instruments' || undefined} aria-busy={busy === 'instruments' || undefined}>
            {copy.refreshInstruments}
          </Button>
          <Button variant="danger" onClick={() => setClearing(true)}>{copy.clearCandles}</Button>
        </div>
      </SettingSection>

      <SettingSection title={copy.backup}>
        <Toggle label={copy.includeCandles(formatBytes(stats?.candleBytes))} checked={includeCandles} onChange={(on) => void toggleSetting(setIncludeCandles(on))} />
        <div className="settings__actions">
          <Button variant="primary" onClick={() => void backupNow()} aria-disabled={backupRunning || undefined} aria-busy={backupRunning || undefined}>
            {copy.backupNow}
          </Button>
        </div>
        <p className="muted" role="status">{backupStatus}</p>
        <Toggle label={copy.scheduled} description={copy.scheduledHint} checked={scheduled} onChange={(on) => void toggleSetting(setScheduled(on))} />
        {scheduled && (
          <>
            <dl className="facts"><Fact label={copy.folder}>{folder || copy.noFolder}</Fact></dl>
            <div className="settings__actions"><Button onClick={() => void chooseScheduledFolder()}>{copy.chooseFolder}</Button></div>
            <KeepField />
          </>
        )}
      </SettingSection>

      <SettingSection title={copy.restore}>
        <p className="muted">{copy.restoreIntro}</p>
        <div className="settings__actions"><Button onClick={() => void startRestore()}>{copy.restoreButton}</Button></div>
      </SettingSection>

      <ConfirmDialog
        open={clearing}
        title={copy.clearTitle}
        message={copy.clearMessage}
        confirmLabel={copy.clearCandles}
        tone="danger"
        busy={busy === 'clear'}
        onConfirm={() => void clear()}
        onCancel={() => setClearing(false)}
      />
      {manifest && <RestoreDialog manifest={manifest} onClose={() => setManifest(null)} />}
    </>
  );
}
