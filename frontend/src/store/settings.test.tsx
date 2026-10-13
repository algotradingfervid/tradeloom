import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { BackendProvider } from '../api/BackendContext';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { suspendSettingsWrites, useSetting, useSettingsStore, writeSetting } from './settings';

function Harness() {
  const [keep, setKeep] = useSetting<number>('backup.schedule.keep', 7);
  const [result, setResult] = useState('');
  return (
    <>
      <output data-testid="value">{String(keep)}</output>
      <button onClick={async () => setResult((await setKeep(0))?.reason ?? 'ok')}>bad</button>
      <button onClick={async () => setResult((await setKeep(30))?.reason ?? 'ok')}>good</button>
      <output data-testid="result">{result}</output>
    </>
  );
}

function setup() {
  const backend = createMockBackend(demoSeed());
  render(<BackendProvider backend={backend}><Harness /></BackendProvider>);
  return { backend, user: userEvent.setup() };
}

describe('useSetting', () => {
  it('commits a valid value to the backend', async () => {
    const { backend, user } = setup();
    await user.click(screen.getByText('good'));
    expect(await screen.findByTestId('result')).toHaveTextContent('ok');
    expect(screen.getByTestId('value')).toHaveTextContent('30');
    expect(backend.state.settings['backup.schedule.keep']).toBe('30');
  });
  it('keeps the old value and returns the reason when the backend refuses (FR-ACCT-012 AC2 pattern)', async () => {
    const { user } = setup();
    await user.click(screen.getByText('bad'));
    // Go service.SetSetting: invalid_setting with the settings.ValidationError text (settings: <key>: <reason>).
    expect(await screen.findByText('settings: backup.schedule.keep: 0 is outside 1…365')).toBeInTheDocument();
    expect(screen.getByTestId('value')).toHaveTextContent('7');
  });
  it('does not call the backend while writes are suspended', async () => {
    const { backend, user } = setup();
    suspendSettingsWrites();
    await user.click(screen.getByText('good'));
    expect(await screen.findByText('A restore is in progress — settings changes are paused')).toBeInTheDocument();
    expect(backend.callsTo('settings.set')).toHaveLength(0);
  });
});

describe('writeSetting rollback guard (Ruling 83-F M4)', () => {
  it('does not roll back over a newer value that arrived while the refused write was in flight', async () => {
    const backend = createMockBackend(demoSeed());
    useSettingsStore.getState().hydrate([{ name: 'backup.schedule.keep', valueJson: '7' }]);
    const pending = writeSetting(backend, 'backup.schedule.keep', 0);
    useSettingsStore.getState().put('backup.schedule.keep', 14); // settings.changed from another writer
    expect(await pending).toMatchObject({ code: 'invalid_setting', stage: 'settings', reason: 'settings: backup.schedule.keep: 0 is outside 1…365' });
    expect(useSettingsStore.getState().values['backup.schedule.keep']).toBe(14);
  });
  it('still rolls back when nothing else changed the key', async () => {
    const backend = createMockBackend(demoSeed());
    useSettingsStore.getState().hydrate([{ name: 'backup.schedule.keep', valueJson: '7' }]);
    await writeSetting(backend, 'backup.schedule.keep', 0);
    expect(useSettingsStore.getState().values['backup.schedule.keep']).toBe(7);
  });
});
