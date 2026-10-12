import { act, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { JobProgress } from '../../api/backend';
import { demoSeed } from '../../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../../api/mock/index';
import { useJobsStore } from '../../store/jobs';
import { useSettingsStore, writeSetting } from '../../store/settings';
import { useToastStore } from '../../store/toasts';
import { renderApp } from '../../test/render';

// Go texts (internal/service/data.go).
const GO_SCHEMA_TOO_NEW = 'This backup was made by a newer version (schema 3; this build supports 1).';
const GO_BACKFILL_RUNNING = '2 candle downloads are still running. Clear the cache when they have finished.';
const GO_JOB_RUNNING = 'Another job (backup) is still running.';
const GO_RESTORE_FAILED = 'The restore failed; your previous data will be put back when Tradeloom restarts: deployments_active: stop every live strategy before restoring';
const ARCHIVE = '/home/asha/Backups/tradeloom-backup-20261011-170000.tar.zst';

type User = ReturnType<typeof renderApp>['user'];

const openRestore = async (user: User) => {
  await user.click(await screen.findByRole('button', { name: 'Restore from a backup…' }));
  return screen.findByRole('dialog', { name: 'Restore this backup?' });
};

const confirmRestore = async (user: User) => {
  const dialog = await openRestore(user);
  await user.type(within(dialog).getByLabelText('Type RESTORE to confirm'), 'RESTORE');
  await user.click(within(dialog).getByRole('button', { name: 'Restore' }));
};

const toastMessages = () => {
  const s = useToastStore.getState();
  return [...s.visible, ...s.queued].map((t) => t.message);
};

const job = (p: Partial<JobProgress>): JobProgress => ({
  jobId: 'job-1', kind: 'backup', stage: 'done', done: 4, total: 4, finished: true, error: '', resultPath: '', ...p,
});

const setsFor = (backend: MockBackend, key: string) => backend.callsTo('settings.set').filter((c) => c.args[0] === key);

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

  it('Clear cached candles refused by Go (backfill_running) shows the reason and keeps the size', async () => {
    const seed = demoSeed();
    seed.stats.backfillOpenJobs = 2;
    const { user } = renderApp({ route: '/settings/data', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Clear cached candles' }));
    await user.click(within(screen.getByRole('dialog', { name: 'Clear cached candles?' })).getByRole('button', { name: 'Clear cached candles' }));
    await waitFor(() => expect(toastMessages()).toContain(GO_BACKFILL_RUNNING));
    expect(screen.getByText('700.0 MB of 10.0 GB')).toBeInTheDocument();
  });

  it('Back up now asks for a folder, honours "include candles" (setting backup.include_candles) and reports the archive', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    await user.click(await screen.findByRole('switch', { name: /Include the candle store/ }));
    await waitFor(() => expect(backend.state.settings['backup.include_candles']).toBe('true'));
    await user.click(screen.getByRole('button', { name: 'Back up now' }));
    expect(backend.callsTo('data.backupNow')[0].args).toEqual(['/home/asha/Backups', true]);
    act(() => backend.emit('job.progress', job({
      jobId: 'job-1', resultPath: '/home/asha/Backups/tradeloom-backup-20261012-103000.tar.zst',
    })));
    expect(await screen.findByText('Backup saved to /home/asha/Backups/tradeloom-backup-20261012-103000.tar.zst')).toBeInTheDocument();
  });

  it('Ruling 86: a backup that fails before its id comes back is reported once, by the Data group (claim window)', async () => {
    const backend = createMockBackend(demoSeed());
    const original = backend.data.backupNow;
    backend.data.backupNow = async (folder, candles) => {
      const id = await original(folder, candles);
      backend.emit('job.progress', job({ jobId: id, stage: 'writing', done: 2, error: 'disk full' }));
      return id;
    };
    const { user } = renderApp({ route: '/settings/data', backend });
    await user.click(await screen.findByRole('button', { name: 'Back up now' }));
    expect(await screen.findByText('Backup failed: disk full')).toBeInTheDocument();
    expect(toastMessages().filter((m) => m.includes('disk full'))).toEqual(['Backup failed: disk full']);
    expect(screen.getByRole('button', { name: 'Back up now' })).toBeEnabled();
  });

  it('a Back up now refusal (invalid_folder) shows Go\'s reason and leaves no job pending', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('data.backupNow', 'Choose an existing folder for the backup.', 'invalid_folder');
    const { user } = renderApp({ route: '/settings/data', backend });
    await user.click(await screen.findByRole('button', { name: 'Back up now' }));
    expect(await screen.findByText('Choose an existing folder for the backup.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back up now' })).toBeEnabled();
    // The claim window is closed: an unclaimed scheduled-backup failure is toasted again.
    act(() => backend.emit('job.progress', job({ jobId: 'backup-9', error: 'The scheduled backup failed: x' })));
    expect(toastMessages()).toContain('The scheduled backup failed: x');
  });

  it('cancelling the folder picker starts nothing', async () => {
    const seed = demoSeed();
    seed.folderChoice = '';
    const { user, backend } = renderApp({ route: '/settings/data', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Back up now' }));
    expect(backend.callsTo('data.backupNow')).toHaveLength(0);
  });

  it('carry-forward: a quit-time scheduled backup success (unknown id, stage done) is ignored gracefully', async () => {
    const { backend } = renderApp({ route: '/settings/data' });
    await screen.findByText('700.0 MB of 10.0 GB');
    act(() => backend.emit('job.progress', job({ jobId: 'backup-7', resultPath: '/home/asha/Backups/tradeloom-backup-x.tar.zst' })));
    expect(toastMessages()).toEqual([]);
    expect(useJobsStore.getState().jobs['backup-7']?.stage).toBe('done');
    expect(screen.getByRole('button', { name: 'Back up now' })).toBeEnabled();
  });

  it('the jobs store is fed by the single existing job.progress listener', async () => {
    const { backend } = renderApp({ route: '/settings/data' });
    await screen.findByText('700.0 MB of 10.0 GB');
    expect(backend.listenerCount('job.progress')).toBe(1);
    act(() => backend.emit('job.progress', job({ jobId: 'x-1', finished: false, stage: 'writing', done: 2 })));
    expect(useJobsStore.getState().jobs['x-1']).toMatchObject({ stage: 'writing', done: 2, finished: false });
  });

  it('scheduled backup is off by default; on reveals folder and retention (7) with inline validation', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    const toggle = await screen.findByRole('switch', { name: 'Scheduled daily backup' });
    expect(toggle).toHaveAttribute('aria-checked', 'false');
    await user.click(toggle);
    await waitFor(() => expect(backend.state.settings['backup.schedule.enabled']).toBe('true'));
    expect(screen.getByText('Not chosen — the backups folder inside the data folder')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Choose folder…' }));
    expect(await screen.findByText('/home/asha/Backups')).toBeInTheDocument();
    await waitFor(() => expect(backend.state.settings['backup.schedule.folder']).toBe('"/home/asha/Backups"'));
    const keep = screen.getByLabelText('Backups to keep');
    expect(keep).toHaveValue('7');
    await user.clear(keep);
    await user.type(keep, '0');
    await user.tab();
    expect(await screen.findByText('Keep between 1 and 365 backups')).toBeInTheDocument();
    expect(setsFor(backend, 'backup.schedule.keep')).toHaveLength(0);
    await user.clear(keep);
    await user.type(keep, '30');
    await user.tab();
    await waitFor(() => expect(setsFor(backend, 'backup.schedule.keep').map((c) => c.args[1])).toEqual(['30']));
  });

  it('restore lists what will be replaced, warns about .replaced-<ts>, secrets.enc and earlier backups, and needs the typed confirmation', async () => {
    const { user, backend } = renderApp({ route: '/settings/data' });
    const dialog = await openRestore(user);
    expect(within(dialog).getByText('Watchlists: 3')).toBeInTheDocument();
    expect(within(dialog).getByText('Strategies: 0')).toBeInTheDocument();
    expect(within(dialog).getByText(/renamed to end in “\.replaced-” and the date and time/)).toBeInTheDocument();
    expect(within(dialog).getByText(/backups in the default backups folder inside the data folder move there with it/)).toBeInTheDocument();
    expect(within(dialog).getByText(/secrets\.enc.*stays in the “\.replaced-” folder/)).toBeInTheDocument();
    const confirm = within(dialog).getByRole('button', { name: 'Restore' });
    expect(confirm).toBeDisabled();
    await user.type(within(dialog).getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(confirm);
    expect(backend.callsTo('data.restore')[0].args).toEqual([ARCHIVE, 'RESTORE']);
    act(() => backend.emit('job.progress', job({ kind: 'restore', stage: 'restart', resultPath: '/home/asha/.local/share/tradeloom.replaced-20261012T103000Z' })));
    expect(await screen.findByText('Restore complete. Tradeloom is restarting…')).toBeInTheDocument();
  });

  it('FR-DATA-005 AC2 (UI part): an archive from a newer schema is refused with Go\'s reason', async () => {
    const seed = demoSeed();
    seed.backup = { ...seed.backup, ok: false, schema: 3, reason: GO_SCHEMA_TOO_NEW };
    const { user } = renderApp({ route: '/settings/data', backend: createMockBackend(seed) });
    await user.click(await screen.findByRole('button', { name: 'Restore from a backup…' }));
    const dialog = await screen.findByRole('dialog', { name: 'This backup cannot be restored' });
    expect(within(dialog).getByRole('alert')).toHaveTextContent(GO_SCHEMA_TOO_NEW);
    expect(within(dialog).queryByRole('button', { name: 'Restore' })).toBeNull();
  });

  it('a restore Go refuses before starting (job_running) shows the reason and re-enables settings writes', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('data.restore', GO_JOB_RUNNING, 'job_running');
    const { user } = renderApp({ route: '/settings/data', backend });
    await confirmRestore(user);
    const dialog = screen.getByRole('dialog', { name: 'Restore this backup?' });
    expect(await within(dialog).findByRole('alert')).toHaveTextContent(GO_JOB_RUNNING);
    expect(await writeSetting(backend, 'appearance.theme', 'dark')).toBeNull();
  });

  it('review focus: once a restore starts no settings write reaches the backend; a failed restore resumes writes and reloads settings', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/settings/data', backend });
    await confirmRestore(user);
    expect(await screen.findByRole('dialog', { name: 'Restoring backup' })).toBeInTheDocument();
    const setsBefore = backend.callsTo('settings.set').length;
    const suspended = await writeSetting(backend, 'appearance.theme', 'dark'); // e.g. a field blurred by the dialog
    expect(suspended?.code).toBe('suspended');
    expect(backend.callsTo('settings.set')).toHaveLength(setsBefore);
    const allBefore = backend.callsTo('settings.all').length;
    // Defensive path: Go ends every started restore with stage "restart" (Ruling 74), but a failure that does not
    // restart must not leave settings writes paused.
    act(() => backend.emit('job.progress', job({ kind: 'restore', stage: 'restoring', done: 3, error: 'Disk full while extracting' })));
    expect((await screen.findAllByText(/Disk full while extracting/)).length).toBeGreaterThan(0);
    await waitFor(() => expect(backend.callsTo('settings.all')).toHaveLength(allBefore + 1));
    expect(await writeSetting(backend, 'appearance.theme', 'dark')).toBeNull();
  });

  it('Rulings 74/75: a restore that fails after closing ends in "restart" — the reason shows, the app restarts, writes stay paused', async () => {
    const backend = createMockBackend(demoSeed());
    const { user } = renderApp({ route: '/settings/data', backend });
    await confirmRestore(user);
    const running = await screen.findByRole('dialog', { name: 'Restoring backup' });
    act(() => backend.emit('job.progress', job({ kind: 'restore', stage: 'closing', done: 2, finished: false })));
    expect(within(running).getByText('Restoring… (2 of 4)')).toBeInTheDocument();
    const allBefore = backend.callsTo('settings.all').length;
    act(() => backend.emit('job.progress', job({ kind: 'restore', stage: 'restart', error: GO_RESTORE_FAILED })));
    expect(await within(running).findByRole('alert')).toHaveTextContent(`Restore failed: ${GO_RESTORE_FAILED}`);
    expect(within(running).getByText('Tradeloom is restarting…')).toBeInTheDocument();
    expect(useSettingsStore.getState().writesSuspended).toBe(true);
    expect(backend.callsTo('settings.all')).toHaveLength(allBefore);
  });
});

describe('Settings → Data: nothing sensitive leaves the dialog (security)', () => {
  it('the typed confirmation and archive paths stay out of web storage and the URL; the jobs store holds only job progress', async () => {
    window.localStorage.clear();
    window.sessionStorage.clear();
    const backend = createMockBackend(demoSeed());
    const { user, router } = renderApp({ route: '/settings/data', backend });
    await user.click(await screen.findByRole('button', { name: 'Back up now' }));
    await confirmRestore(user);
    act(() => backend.emit('job.progress', job({ kind: 'restore', jobId: 'job-2', stage: 'restart' })));
    await screen.findByText('Restore complete. Tradeloom is restarting…');
    const storage = JSON.stringify({ ...window.localStorage }) + JSON.stringify({ ...window.sessionStorage });
    expect(storage).not.toContain('RESTORE');
    expect(storage).not.toContain(ARCHIVE);
    expect(router.state.location.pathname + router.state.location.search).toBe('/settings/data');
    for (const p of Object.values(useJobsStore.getState().jobs)) {
      expect(Object.keys(p).sort()).toEqual(['done', 'error', 'finished', 'jobId', 'kind', 'resultPath', 'stage', 'total']);
    }
    expect(JSON.stringify(useJobsStore.getState())).not.toContain('RESTORE');
  });
});
