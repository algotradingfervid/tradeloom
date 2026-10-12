> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 16.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 16: Settings → Data — storage facts, clear candles, backup, scheduled backup, restore

**Files:**
- Create: `frontend/src/store/jobs.ts`, `frontend/src/views/settings/DataGroup.tsx`, `frontend/e2e/settings-data.spec.ts`
- Modify: `frontend/src/i18n/format.ts` (`formatBytes`), `frontend/src/api/events.ts` (`job.progress`), `frontend/src/views/settings/SettingsView.tsx` (`GROUP_COMPONENTS.data`), `frontend/src/i18n/copy.ts`
- Test: `frontend/src/i18n/formatBytes.test.ts`, `frontend/src/views/settings/DataGroup.test.tsx`

**Interfaces:**
- Consumes: Task 2 `Backend.data.{stats,clearCandles,refreshInstruments,chooseFolder,chooseBackupFile,backupNow,inspectBackup,restore}`, `DataStats{dataDir, appDbBytes, candleBytes, candleBudgetBytes, instrumentsImportedAtMs, backfillOpenJobs}`, `BackupInfo` (`counts` keyed `strategies`, `scanners`, `backtest_runs`, `deployments`, `watchlists`, `order_journal`; `createdAt` RFC3339), `JobProgress{jobId, kind, stage, done, total, finished, error, resultPath}`, `timeMs`, `suspendSettingsWrites`, `resumeSettingsWrites`, `reloadSettings`, `writeSetting`, `useSetting` on `backup.schedule.enabled`, `backup.schedule.folder`, `backup.schedule.keep`, `backup.include_candles`; Task 14 `SettingSection`, `Fact`, `SettingNumberField`; Task 3 `ConfirmDialog`, `Dialog`, `Toggle`; canonical fact A5 (restore job ends with `stage: "restart"`, Go exits with code 3 and `main` re-executes), P0-1 FR-DATA-005 engine.
- Produces: `formatBytes(v): string` (binary units, `0 B`, `700.0 MB`, `1.50 KB`); `useJobsStore` (`jobs`, `apply`); `DataGroup()`; `RESTORE_PHRASE = 'RESTORE'`.

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/i18n/formatBytes.test.ts`:

```ts
import { expect, it } from 'vitest';
import { DASH, formatBytes } from './format';

it('formats sizes in binary units with Indian grouping', () => {
  expect(formatBytes(0)).toBe('0 B');
  expect(formatBytes(1536)).toBe('1.50 KB');
  expect(formatBytes(734_003_200)).toBe('700.0 MB');
  expect(formatBytes(10_737_418_240)).toBe('10.0 GB');
  expect(formatBytes(-1)).toBe(DASH);
  expect(formatBytes(undefined)).toBe(DASH);
});
```

Create `frontend/src/views/settings/DataGroup.test.tsx`:

```tsx
import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { writeSetting } from '../../store/settings';
import { renderApp } from '../../test/render';

const openRestore = async (user: ReturnType<typeof renderApp>['user']) => {
  await user.click(await screen.findByRole('button', { name: 'Restore from a backup…' }));
  return screen.findByRole('dialog', { name: 'Restore this backup?' });
};

describe('Settings → Data (FR-ACCT-012, FR-DATA-005 UI)', () => {
  it('shows the data directory and the candle store size against its budget', async () => {
    renderApp({ route: '/settings/data' });
    expect(await screen.findByText('700.0 MB of 10.0 GB')).toBeInTheDocument();
    expect(screen.getByText('/home/asha/.local/share/tradeloom')).toBeInTheDocument();
  });
  it('FR-ACCT-012 AC1 (UI part): Clear cached candles → confirm → the store size drops to 0 B', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    await user.click(await screen.findByRole('button', { name: 'Clear cached candles' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Clear cached candles?' })).getByRole('button', { name: 'Clear cached candles' }));
    expect(backend.callsTo('data.clearCandles')).toHaveLength(1);
    expect(await screen.findByText('0 B of 10.0 GB')).toBeInTheDocument();
  });
  it('Back up now asks for a folder, honours "include candles" (setting backup.include_candles) and reports the archive', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    await user.click(await screen.findByRole('switch', { name: /Include the candle store/ }));
    await waitFor(() => expect(backend.state.settings['backup.include_candles']).toBe('true'));
    await user.click(screen.getByRole('button', { name: 'Back up now' }));
    expect(backend.callsTo('data.backupNow')[0].args).toEqual(['/home/asha/Backups', true]);
    act(() => backend.emit('job.progress', {
      jobId: 'job-1', kind: 'backup', stage: 'done', done: 1, total: 1, finished: true, error: '',
      resultPath: '/home/asha/Backups/tradeloom-backup-20261012-103000.tar.zst',
    }));
    expect(await screen.findByText('Backup saved to /home/asha/Backups/tradeloom-backup-20261012-103000.tar.zst')).toBeInTheDocument();
  });
  it('cancelling the folder picker starts nothing', async () => {
    const seed = demoSeed();
    seed.folderChoice = '';
    const { user, backend } = renderApp({ route: '/settings/data', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Back up now' }));
    expect(backend.callsTo('data.backupNow')).toHaveLength(0);
  });
  it('scheduled backup is off by default; on reveals folder and retention (7) with inline validation', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    const toggle = await screen.findByRole('switch', { name: 'Scheduled daily backup' });
    expect(toggle).toHaveAttribute('aria-checked', 'false');
    await user.click(toggle);
    await waitFor(() => expect(backend.state.settings['backup.schedule.enabled']).toBe('true'));
    await user.click(screen.getByRole('button', { name: 'Choose folder…' }));
    expect(await screen.findByText('/home/asha/Backups')).toBeInTheDocument();
    const keep = screen.getByLabelText('Backups to keep');
    expect(keep).toHaveValue('7');
    await user.clear(keep);
    await user.type(keep, '0');
    await user.tab();
    expect(await screen.findByText('Keep between 1 and 365 backups')).toBeInTheDocument();
  });
  it('restore lists what will be replaced and needs the typed confirmation', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    const dialog = await openRestore(user);
    expect(within(dialog).getByText('Watchlists: 3')).toBeInTheDocument();
    expect(within(dialog).getByText('Strategies: 0')).toBeInTheDocument();
    const confirm = within(dialog).getByRole('button', { name: 'Restore' });
    expect(confirm).toBeDisabled();
    await user.type(within(dialog).getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(confirm);
    expect(backend.callsTo('data.restore')[0].args).toEqual(['/home/asha/Backups/tradeloom-backup-20261011-170000.tar.zst', 'RESTORE']);
    act(() => backend.emit('job.progress', { jobId: 'job-1', kind: 'restore', stage: 'restart', done: 3, total: 3, finished: true, error: '', resultPath: '' }));
    expect(await screen.findByText('Restore complete. Tradeloom is restarting…')).toBeInTheDocument();
  });
  it('FR-DATA-005 AC2 (UI part): an archive from a newer schema is refused with the reason', async () => {
    const seed = demoSeed();
    seed.backup = { ...seed.backup, ok: false, schema: 3, reason: 'Backup schema 3 (written by v1.4.0) is newer than this version supports (1)' };
    const { user } = renderApp({ route: '/settings/data', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Restore from a backup…' }));
    const dialog = await screen.findByRole('dialog', { name: 'This backup cannot be restored' });
    expect(within(dialog).getByRole('alert')).toHaveTextContent('Backup schema 3 (written by v1.4.0) is newer than this version supports (1)');
    expect(within(dialog).queryByRole('button', { name: 'Restore' })).toBeNull();
  });
  it('a restore the backend refuses (deployments running) shows the reason and re-enables settings writes', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('data.restore', 'Stop all deployments before restoring', 'deployments_active');
    const { user } = renderApp({ route: '/settings/data', backend });
    const dialog = await openRestore(user);
    await user.type(within(dialog).getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(within(dialog).getByRole('button', { name: 'Restore' }));
    expect(await screen.findByText('Stop all deployments before restoring')).toBeInTheDocument();
    expect(await writeSetting(backend, 'appearance.theme', 'dark')).toBeNull();
  });
  it('review focus: once a restore starts no settings write reaches the backend; a failed restore resumes writes and reloads settings', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/settings/data', backend });
    const dialog = await openRestore(user);
    await user.type(within(dialog).getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(within(dialog).getByRole('button', { name: 'Restore' }));
    expect(await screen.findByRole('dialog', { name: 'Restoring backup' })).toBeInTheDocument();
    const setsBefore = backend.callsTo('settings.set').length;
    const suspended = await writeSetting(backend, 'appearance.theme', 'dark'); // e.g. a field blurred by the dialog
    expect(suspended?.code).toBe('suspended');
    expect(backend.callsTo('settings.set')).toHaveLength(setsBefore);
    const allBefore = backend.callsTo('settings.all').length;
    act(() => backend.emit('job.progress', { jobId: 'job-1', kind: 'restore', stage: 'extract', done: 1, total: 3, finished: true, error: 'Disk full while extracting', resultPath: '' }));
    expect((await screen.findAllByText(/Disk full while extracting/)).length).toBeGreaterThan(0);
    await waitFor(() => expect(backend.callsTo('settings.all')).toHaveLength(allBefore + 1));
    expect(await writeSetting(backend, 'appearance.theme', 'dark')).toBeNull();
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/i18n/formatBytes.test.ts src/views/settings/DataGroup.test.tsx`
Expected: FAIL — `formatBytes` is not exported; `/settings/data` renders "Page not found".

- [ ] **Step 3: Implement sizes, the jobs store and the Data group**

Append to `frontend/src/i18n/format.ts`:

```ts
const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

/** Binary units (1 KB = 1024 B): two decimals below 10, one decimal otherwise. */
export function formatBytes(v: Num): string {
  if (!isNum(v) || v < 0) return DASH;
  let n = v;
  let i = 0;
  while (n >= 1024 && i < BYTE_UNITS.length - 1) {
    n /= 1024;
    i += 1;
  }
  if (i === 0) return `${grouping(0).format(n)} B`;
  const digits = n < 10 ? 2 : 1;
  return `${grouping(digits).format(roundTo(n, digits))} ${BYTE_UNITS[i]}`;
}
```

Create `frontend/src/store/jobs.ts`:

```ts
import { create } from 'zustand';
import type { JobProgress } from '../api/backend';
import { registerReset } from './reset';

interface JobsStore { jobs: Record<string, JobProgress>; apply(p: JobProgress): void }

export const useJobsStore = create<JobsStore>()((set) => ({
  jobs: {},
  apply: (p) => set((s) => ({ jobs: { ...s.jobs, [p.jobId]: p } })),
}));

registerReset(() => useJobsStore.setState({ jobs: {} }));
```

In `frontend/src/api/events.ts` add `import { useJobsStore } from '../store/jobs';` and append to the `offs` array:

```ts
    backend.on('job.progress', (p) => useJobsStore.getState().apply(p)),
```

Append to `frontend/src/i18n/copy.ts`:

```ts
export const dataCopy = {
  storage: 'Storage',
  dataDir: 'Data folder',
  appDb: 'App database',
  candles: 'Candle store',
  candleUsage: (used: string, budget: string) => `${used} of ${budget}`,
  instruments: 'Instrument list updated',
  refreshInstruments: 'Refresh instrument list now',
  instrumentsRefreshed: 'Instrument list refreshed',
  backfill: 'History downloads waiting',
  backfillOpen: (n: number) => (n === 0 ? 'None' : n === 1 ? '1 job' : `${n} jobs`),
  clearCandles: 'Clear cached candles',
  clearTitle: 'Clear cached candles?',
  clearMessage: 'All downloaded candles are deleted. Charts download their data again the next time they open. Watchlists, settings and other data are not affected.',
  cleared: 'Cached candles cleared',
  backup: 'Backup',
  includeCandles: (size: string) => `Include the candle store (${size})`,
  backupNow: 'Back up now',
  backingUp: 'Backing up…',
  backupSaved: (path: string) => `Backup saved to ${path}`,
  backupFailed: (reason: string) => `Backup failed: ${reason}`,
  scheduled: 'Scheduled daily backup',
  scheduledHint: 'Runs after 16:00 or when you quit, into the folder below.',
  folder: 'Backup folder',
  noFolder: 'Not chosen yet',
  chooseFolder: 'Choose folder…',
  keep: 'Backups to keep',
  restore: 'Restore',
  restoreIntro: 'Replace all Tradeloom data with the contents of a backup archive. Kite credentials are never part of a backup.',
  restoreButton: 'Restore from a backup…',
  restoreTitle: 'Restore this backup?',
  restoreConfirm: 'Restore',
  restoreFrom: (when: string, version: string) => `Backup made on ${when} by Tradeloom v${version}. It replaces:`,
  count: (label: string, n: number) => `${label}: ${n}`,
  // Keys of the P0-1 backup manifest `counts`.
  counts: { strategies: 'Strategies', scanners: 'Scanners', backtest_runs: 'Backtest runs', deployments: 'Deployments', watchlists: 'Watchlists' },
  withCandles: 'The archive includes the candle store.',
  withoutCandles: 'The archive does not include candles; charts will download history again.',
  restoreWarning: 'Your current data folder is kept next to the restored one. Tradeloom restarts when the restore finishes; settings you are still editing are discarded. On a different computer you will be asked for your Kite credentials.',
  cannotRestore: 'This backup cannot be restored',
  restoring: 'Restoring backup',
  restoreStage: (done: number, total: number) => `Restoring… (${done} of ${total})`,
  restarting: 'Restore complete. Tradeloom is restarting…',
  restoreFailed: (reason: string) => `Restore failed: ${reason}`,
} as const;
```

Create `frontend/src/views/settings/DataGroup.tsx`:

```tsx
import { useCallback, useEffect, useState } from 'react';
import { timeMs, type BackupInfo, type DataStats } from '../../api/backend';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { Dialog } from '../../components/Dialog';
import { Toggle } from '../../components/Toggle';
import { componentCopy, dataCopy as copy } from '../../i18n/copy';
import { DASH, formatBytes, formatDateTimeLong } from '../../i18n/format';
import { useJobsStore } from '../../store/jobs';
import { reloadSettings, resumeSettingsWrites, suspendSettingsWrites, useSetting } from '../../store/settings';
import { pushToast } from '../../store/toasts';
import { Fact, SettingNumberField, SettingSection } from './SettingRow';

export const RESTORE_PHRASE = 'RESTORE';

const fail = (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason });

const COUNT_KEYS = ['strategies', 'scanners', 'backtest_runs', 'deployments', 'watchlists'] as const;

function RestoreSummary({ manifest }: { manifest: BackupInfo }) {
  return (
    <>
      <p>{copy.restoreFrom(formatDateTimeLong(timeMs(manifest.createdAt)), manifest.appVersion)}</p>
      <ul>
        {COUNT_KEYS.map((k) => <li key={k}>{copy.count(copy.counts[k], manifest.counts[k] ?? 0)}</li>)}
      </ul>
      <p>{manifest.includesCandles ? copy.withCandles : copy.withoutCandles}</p>
      <p>{copy.restoreWarning}</p>
    </>
  );
}

function RestoreDialog({ manifest, onClose }: { manifest: BackupInfo; onClose: () => void }) {
  const backend = useBackend();
  const [jobId, setJobId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const job = useJobsStore((s) => (jobId ? s.jobs[jobId] : undefined));

  useEffect(() => {
    if (!job?.finished || !job.error) return;
    const reason = job.error;
    setJobId(null);
    setError(reason);
    resumeSettingsWrites();
    void reloadSettings(backend);
    pushToast({ level: 'error', message: copy.restoreFailed(reason) });
  }, [job, backend]);

  const start = async () => {
    setError(null);
    suspendSettingsWrites(); // review focus #5: nothing may write over the database being restored
    try {
      setJobId(await backend.data.restore(manifest.path, RESTORE_PHRASE));
    } catch (e) {
      resumeSettingsWrites();
      setError(parseBackendError(e).reason);
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
    // A successful restore ends with stage "restart": Go quits with exit code 3 and `main` starts the app again (A5).
    const done = job?.finished === true && !job.error;
    return (
      <Dialog open title={copy.restoring} size="sm" onClose={onClose} dismissible={false}>
        <p role="status">{done ? copy.restarting : copy.restoreStage(job?.done ?? 0, job?.total ?? 0)}</p>
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
      error={error}
      onConfirm={() => void start()}
      onCancel={onClose}
    />
  );
}

export function DataGroup() {
  const backend = useBackend();
  const [stats, setStats] = useState<DataStats | null>(null);
  const [statsError, setStatsError] = useState<string | null>(null);
  const [clearing, setClearing] = useState(false);
  const [includeCandles, setIncludeCandles] = useSetting<boolean>('backup.include_candles', false);
  const [backupJob, setBackupJob] = useState<string | null>(null);
  const backupProgress = useJobsStore((s) => (backupJob ? s.jobs[backupJob] : undefined));
  const [manifest, setManifest] = useState<BackupInfo | null>(null);
  const [scheduled, setScheduled] = useSetting<boolean>('backup.schedule.enabled', false);
  const [folder, setFolder] = useSetting<string>('backup.schedule.folder', '');

  const loadStats = useCallback(async () => {
    try {
      setStats(await backend.data.stats());
      setStatsError(null);
    } catch (e) {
      setStatsError(parseBackendError(e).reason);
    }
  }, [backend]);
  useEffect(() => {
    void loadStats();
  }, [loadStats]);

  useEffect(() => {
    if (!backupProgress?.finished) return;
    if (backupProgress.error) pushToast({ level: 'error', message: copy.backupFailed(backupProgress.error) });
    else pushToast({ level: 'success', message: copy.backupSaved(backupProgress.resultPath) });
    setBackupJob(null);
  }, [backupProgress]);

  const clear = async () => {
    try {
      await backend.data.clearCandles();
      pushToast({ level: 'success', message: copy.cleared });
      await loadStats();
    } catch (e) {
      fail(e);
    } finally {
      setClearing(false);
    }
  };
  const refreshInstruments = async () => {
    try {
      await backend.data.refreshInstruments();
      pushToast({ level: 'success', message: copy.instrumentsRefreshed });
      await loadStats();
    } catch (e) {
      fail(e);
    }
  };
  const backupNow = async () => {
    try {
      const target = await backend.data.chooseFolder();
      if (!target) return;
      setBackupJob(await backend.data.backupNow(target, includeCandles));
    } catch (e) {
      fail(e);
    }
  };
  const chooseScheduledFolder = async () => {
    try {
      const target = await backend.data.chooseFolder();
      if (target) await setFolder(target);
    } catch (e) {
      fail(e);
    }
  };
  const startRestore = async () => {
    try {
      const path = await backend.data.chooseBackupFile();
      if (!path) return;
      setManifest(await backend.data.inspectBackup(path));
    } catch (e) {
      fail(e);
    }
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
          <Button onClick={() => void refreshInstruments()}>{copy.refreshInstruments}</Button>
          <Button variant="danger" onClick={() => setClearing(true)}>{copy.clearCandles}</Button>
        </div>
      </SettingSection>

      <SettingSection title={copy.backup}>
        <Toggle label={copy.includeCandles(formatBytes(stats?.candleBytes))} checked={includeCandles} onChange={(on) => void setIncludeCandles(on)} />
        <div className="settings__actions">
          <Button variant="primary" onClick={() => void backupNow()} disabled={backupJob !== null}>{backupJob ? copy.backingUp : copy.backupNow}</Button>
        </div>
        <Toggle label={copy.scheduled} description={copy.scheduledHint} checked={scheduled} onChange={(on) => void setScheduled(on)} />
        {scheduled && (
          <>
            <dl className="facts"><Fact label={copy.folder}>{folder || copy.noFolder}</Fact></dl>
            <div className="settings__actions"><Button onClick={() => void chooseScheduledFolder()}>{copy.chooseFolder}</Button></div>
            <SettingNumberField settingKey="backup.schedule.keep" label={copy.keep} fallback={7} />
          </>
        )}
      </SettingSection>

      <SettingSection title={copy.restore}>
        <p className="muted">{copy.restoreIntro}</p>
        <div className="settings__actions"><Button onClick={() => void startRestore()}>{copy.restoreButton}</Button></div>
      </SettingSection>

      <ConfirmDialog open={clearing} title={copy.clearTitle} message={copy.clearMessage} confirmLabel={copy.clearCandles} tone="danger" onConfirm={() => void clear()} onCancel={() => setClearing(false)} />
      {manifest && <RestoreDialog manifest={manifest} onClose={() => setManifest(null)} />}
    </>
  );
}
```

In `frontend/src/views/settings/SettingsView.tsx` add `import { DataGroup } from './DataGroup';` and `data: DataGroup,` to `GROUP_COMPONENTS`.

- [ ] **Step 4: Run the suite to verify it passes**

Run: `cd frontend && npx vitest run && npm run typecheck`
Expected: PASS (41 test files), `tsc` exits 0.

- [ ] **Step 5: Add the backup/restore Playwright path and run it**

Create `frontend/e2e/settings-data.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('Settings → Data: Back up now and Restore against the mock job runner (FR-DATA-005 UI path)', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('button', { name: 'Account menu' })).toBeVisible();
  await page.evaluate(() => window.__tradeloomMock?.emit('app.navigate', { route: '/settings/data' }));
  await expect(page.getByText('700.0 MB of 10.0 GB')).toBeVisible();
  await page.getByRole('button', { name: 'Back up now' }).click();
  await expect(page.getByText(/^Backup saved to \/home\/asha\/Backups\//)).toBeVisible();
  await page.getByRole('button', { name: 'Restore from a backup…' }).click();
  const dialog = page.getByRole('dialog', { name: 'Restore this backup?' });
  await dialog.getByLabel('Type RESTORE to confirm').fill('RESTORE');
  await dialog.getByRole('button', { name: 'Restore' }).click();
  await expect(page.getByText('Restore complete. Tradeloom is restarting…')).toBeVisible();
});
```

Run: `cd frontend && npx playwright test e2e/settings-data.spec.ts`
Expected: PASS (1 test × 2 projects: webkit, chromium).

- [ ] **Step 6: Commit**

```bash
git add frontend/src frontend/e2e
git commit -m "feat(frontend): Settings → Data with backup, scheduled backup and guarded restore (P0-4 Task 16)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
