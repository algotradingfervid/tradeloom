import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { demoSeed, FIXED_NOW } from '../../api/mock/fixtures';
import { createMockBackend } from '../../api/mock/index';
import { useToastStore } from '../../store/toasts';
import { renderApp } from '../../test/render';

const toasts = () => useToastStore.getState().visible.map((t) => `${t.level}: ${t.message}`);
const GO_BUNDLE_FAILED = 'The support bundle could not be written: the exports folder does not exist';

describe('FR-SHELL-018 diagnostics (UI)', () => {
  it('switches the log level at runtime', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    await user.selectOptions(await screen.findByLabelText('Log detail'), 'debug');
    await waitFor(() => expect(backend.state.settings['log.level']).toBe('"debug"'));
    // ui.last_route is also written on mount: filter by key.
    expect(backend.callsTo('settings.set').filter((c) => c.args[0] === 'log.level').map((c) => c.args)).toEqual([['log.level', '"debug"']]);
  });
  it('shows a refused log-level write inline', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    const select = await screen.findByLabelText('Log detail');
    // Armed after mount: ui.last_route is written through settings.set on mount.
    await waitFor(() => expect(backend.callsTo('settings.set').some((c) => c.args[0] === 'ui.last_route')).toBe(true));
    backend.failNext('settings.set', 'Settings database is read-only', 'internal');
    await user.selectOptions(select, 'debug');
    expect(await screen.findByText('Settings database is read-only')).toBeInTheDocument();
    expect(select).toHaveValue('info');
  });
  it('opens the logs folder and creates a support bundle', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    await user.click(await screen.findByRole('button', { name: 'Open logs folder' }));
    expect(backend.callsTo('diagnostics.openLogsFolder')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Create support bundle' }));
    // The path is shown exactly as Go returns it.
    expect(await screen.findByText('Support bundle saved to /home/asha/.local/share/tradeloom/exports/support-20261012-103000.zip')).toBeInTheDocument();
  });
  it('a refused logs-folder open shows Go\'s reason', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    backend.failNext('diagnostics.openLogsFolder', 'The logs folder is not configured.', 'unavailable');
    await user.click(await screen.findByRole('button', { name: 'Open logs folder' }));
    await waitFor(() => expect(toasts()).toEqual(['error: The logs folder is not configured.']));
  });
  it('Ruling 86: a failed bundle is reported once, even when its job.progress failure arrives after the refusal', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    backend.failNext('diagnostics.createSupportBundle', GO_BUNDLE_FAILED, 'bundle_failed');
    await user.click(await screen.findByRole('button', { name: 'Create support bundle' }));
    await waitFor(() => expect(toasts()).toEqual([`error: ${GO_BUNDLE_FAILED}`]));
    // Go publishes job.progress on its own goroutine: the finished event can land after the call returned.
    backend.emit('job.progress', {
      jobId: 'support_bundle-1', kind: 'support_bundle', stage: 'collecting', done: 1, total: 3, finished: true,
      error: 'the exports folder does not exist', resultPath: '',
    });
    expect(toasts()).toEqual([`error: ${GO_BUNDLE_FAILED}`]);
    expect(screen.getByRole('button', { name: 'Create support bundle' })).toBeEnabled();
  });
  it('Ruling 86: another job failing while the bundle call is pending still shows once the call ends', async () => {
    const { user, backend } = renderApp({ route: '/settings/diagnostics' });
    let release: (p: string) => void = () => {};
    backend.diagnostics.createSupportBundle = () => new Promise<string>((r) => { release = r; });
    await user.click(await screen.findByRole('button', { name: 'Create support bundle' }));
    expect(screen.getByRole('button', { name: 'Create support bundle' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Create support bundle' })).toHaveAttribute('aria-busy', 'true');
    backend.emit('job.progress', {
      jobId: 'backup-7', kind: 'backup', stage: 'write', done: 1, total: 4, finished: true,
      error: 'The scheduled backup failed: disk full', resultPath: '',
    });
    expect(toasts()).toEqual([]);
    release('/exports/support-1.zip');
    await waitFor(() => expect(toasts()).toContain('error: The scheduled backup failed: disk full'));
  });
  it('AC2 (UI): after a crash the next start shows "The app crashed last time" with View report', async () => {
    const seed = demoSeed();
    seed.crash = { path: '/home/asha/.local/share/tradeloom/logs/crash-20261012-101500.json', atMs: FIXED_NOW - 900_000 };
    const { user, backend } = renderApp({ backend: createMockBackend(seed) });
    expect(await screen.findByText(/The app crashed last time/)).toBeInTheDocument();
    expect(screen.getByText(/The app crashed last time/)).toHaveTextContent('The app crashed last time (12 Oct 2026, 10:15).');
    await user.click(screen.getByRole('button', { name: 'View report' }));
    expect(backend.callsTo('diagnostics.openCrashReport')).toHaveLength(1);
    await user.click(screen.getByRole('button', { name: 'Dismiss crash notice' }));
    expect(screen.queryByText(/The app crashed last time/)).toBeNull();
    expect(backend.callsTo('diagnostics.dismissCrash')).toHaveLength(1);
  });
  it('a refused View report shows Go\'s reason', async () => {
    const seed = demoSeed();
    seed.crash = { path: '/home/asha/.local/share/tradeloom/logs/crash-20261012-101500.json', atMs: FIXED_NOW - 900_000 };
    const backend = createMockBackend(seed);
    const { user } = renderApp({ backend });
    await screen.findByText(/The app crashed last time/);
    backend.failNext('diagnostics.openCrashReport', 'There is no unseen crash report.', 'no_crash');
    await user.click(screen.getByRole('button', { name: 'View report' }));
    await waitFor(() => expect(toasts()).toEqual(['error: There is no unseen crash report.']));
  });
  it('shows no crash notice after a clean exit', async () => {
    renderApp();
    await screen.findByRole('button', { name: 'Account menu' });
    expect(screen.queryByText(/The app crashed last time/)).toBeNull();
  });
});
