import { describe, expect, it } from 'vitest';
import { colourTokens, cssVarName } from './tokens';
import { contrastRatio } from './contrast';

const FOREGROUNDS = ['text', 'textMuted', 'accent', 'up', 'down', 'warn', 'flat'] as const;
const BACKGROUNDS = ['bg', 'surface', 'surface2'] as const;
const PAIRS = FOREGROUNDS.flatMap((f) => BACKGROUNDS.map((b) => [f, b] as const));

describe.each(['light', 'dark'] as const)('FR-SHELL-011 AC3 — %s theme contrast', (name) => {
  const t = colourTokens[name];
  it.each(PAIRS)('%s on %s ≥ 4.5:1', (fg, bg) => {
    expect(contrastRatio(t[fg], t[bg])).toBeGreaterThanOrEqual(4.5);
  });
  it('onAccent on accent ≥ 4.5:1', () => {
    expect(contrastRatio(t.onAccent, t.accent)).toBeGreaterThanOrEqual(4.5);
  });
});

describe('token plumbing', () => {
  it('both themes define the same keys', () => {
    expect(Object.keys(colourTokens.dark).sort()).toEqual(Object.keys(colourTokens.light).sort());
  });
  it('maps camelCase keys to kebab-case CSS variables', () => {
    expect(cssVarName('surface2')).toBe('--surface-2');
    expect(cssVarName('textMuted')).toBe('--text-muted');
  });
});
