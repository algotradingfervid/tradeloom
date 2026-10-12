import { describe, expect, it, vi } from 'vitest';
import { colourTokens } from '../design/tokens';
import { readChartTheme } from './theme';

describe('readChartTheme', () => {
  it('takes background/text/up/down from the theme tokens and the chart-only colours from CSS variables', () => {
    const vars: Record<string, string> = {
      '--chart-grid': ' #000003', '--chart-crosshair': '#000004', '--chart-separator': '#000005',
      '--chart-volume-up': '#000008', '--chart-volume-down': '#000009',
    };
    // jsdom's cascade support for custom properties is partial; stub the computed style deterministically.
    const spy = vi.spyOn(window, 'getComputedStyle').mockReturnValue({ getPropertyValue: (n: string) => vars[n] ?? '' } as never);
    const el = document.createElement('div');
    const t = colourTokens.dark;
    expect(readChartTheme(t, el)).toEqual({
      background: t.bg, text: t.text, grid: '#000003', crosshair: '#000004', separator: '#000005',
      up: t.up, down: t.down, volumeUp: '#000008', volumeDown: '#000009',
    });
    expect(spy).toHaveBeenCalledWith(el);
    spy.mockRestore();
  });
});
