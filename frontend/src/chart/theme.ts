import type { ColourTokens } from '../design/tokens';
import type { ChartTheme } from './types';

const read = (s: CSSStyleDeclaration, name: string) => s.getPropertyValue(name).trim();

/**
 * Chart colours come from design tokens (no literal colours in src/, ARCHITECTURE §8.6): the shared ones from
 * P0-4's `useTheme().tokens`, the chart-only ones from the `--chart-*` variables in tokens.css.
 */
export function readChartTheme(tokens: ColourTokens, el: HTMLElement = document.documentElement): ChartTheme {
  const s = getComputedStyle(el);
  return {
    background: tokens.bg, text: tokens.text, grid: read(s, '--chart-grid'),
    crosshair: read(s, '--chart-crosshair'), separator: read(s, '--chart-separator'),
    up: tokens.up, down: tokens.down,
    volumeUp: read(s, '--chart-volume-up'), volumeDown: read(s, '--chart-volume-down'),
  };
}
