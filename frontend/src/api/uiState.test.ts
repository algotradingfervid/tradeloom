import { beforeEach, describe, expect, it } from 'vitest';
import { setBackend } from './current';
import { demoSeed } from './mock/fixtures';
import { createMockBackend, type MockBackend } from './mock/index';
import { readUiState, writeUiState } from './uiState';

let backend: MockBackend;
beforeEach(() => { backend = createMockBackend(demoSeed()); setBackend(backend); });

describe('uiState over Settings.GetUiState/SetUiState', () => {
  it('round-trips JSON and returns the fallback when absent', async () => {
    expect(await readUiState('chart.dashboard', null)).toBeNull();
    await writeUiState('chart.dashboard', { identifier: 'NSE:RELIANCE', timeframe: '15m' });
    expect(await backend.settings.getUiState('chart.dashboard')).toBe('{"identifier":"NSE:RELIANCE","timeframe":"15m"}');
    expect(await readUiState('chart.dashboard', null)).toEqual({ identifier: 'NSE:RELIANCE', timeframe: '15m' });
  });
  it('a corrupt stored value falls back instead of throwing', async () => {
    await backend.settings.setUiState('chart.dashboard', '{oops');
    expect(await readUiState('chart.dashboard', 'fallback')).toBe('fallback');
  });
});
