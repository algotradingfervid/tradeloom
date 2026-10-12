import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { BackendProvider } from '../api/BackendContext';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { suspendSettingsWrites, useSetting } from './settings';

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
    expect(await screen.findByText('Keep between 1 and 365 backups')).toBeInTheDocument();
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
