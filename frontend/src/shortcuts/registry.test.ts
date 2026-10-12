import { describe, expect, it } from 'vitest';
import type { ShortcutAction } from '../api/backend';
import { shortcutFixture } from '../api/mock/fixtures';
import { bindingFromEvent, bindingMatches, formatBinding, parseBinding, resolveAction, scopeStackFor } from './registry';

const ev = (key: string, code: string, mods: Partial<{ ctrlKey: boolean; altKey: boolean; shiftKey: boolean; metaKey: boolean }> = {}) => ({
  key, code, ctrlKey: false, altKey: false, shiftKey: false, metaKey: false, ...mods,
});

// Go's real labels (internal/settings/shortcuts.go; Ruling 83-F I3).
const actions: ShortcutAction[] = [
  { id: 'toasts.focus', label: 'Focus toasts', scope: 'global', defaultBinding: 'Alt+T', binding: 'Alt+T' },
  { id: 'chart.trendLine', label: 'Draw trend line', scope: 'chart', defaultBinding: 'Alt+T', binding: 'Alt+T' },
];

describe('shortcut registry', () => {
  it('parses and formats bindings', () => {
    expect(parseBinding('Ctrl+Shift+K')).toEqual({ key: 'K', ctrl: true, alt: false, shift: true, meta: false });
    expect(parseBinding('Alt+↑')?.key).toBe('ArrowUp');
    expect(parseBinding('F11')?.key).toBe('F11');
    expect(parseBinding('')).toBeNull();
    expect(parseBinding('Ctrl+')).toBeNull();
    expect(formatBinding('ctrl+shift+k')).toBe('Ctrl+Shift+K');
  });
  it('matches by physical key so Alt+letter works on any layout', () => {
    expect(bindingMatches('Alt+T', ev('†', 'KeyT', { altKey: true }))).toBe(true);
    expect(bindingMatches('Alt+1', ev('1', 'Digit1', { altKey: true }))).toBe(true);
    expect(bindingMatches('Alt+1', ev('1', 'Digit1', { altKey: true, shiftKey: true }))).toBe(false);
  });
  it('builds a binding from a key event and ignores lone modifiers', () => {
    expect(bindingFromEvent(ev('Shift', 'ShiftLeft', { shiftKey: true }))).toBeNull();
    expect(bindingFromEvent(ev('x', 'KeyX', { ctrlKey: true, shiftKey: true }))).toBe('Ctrl+Shift+X');
  });
  it('innermost scope wins; falls back when that scope has no handler', () => {
    const e = ev('t', 'KeyT', { altKey: true });
    expect(resolveAction(actions, ['global', 'view', 'chart'], e)?.id).toBe('chart.trendLine');
    expect(resolveAction(actions, ['global', 'view'], e)?.id).toBe('toasts.focus');
    expect(resolveAction(actions, ['global', 'view', 'chart'], e, (id) => id !== 'chart.trendLine')?.id).toBe('toasts.focus');
  });
  it('derives the scope stack from data-shortcut-scope ancestors', () => {
    const outer = document.createElement('div');
    outer.dataset.shortcutScope = 'chart';
    const inner = document.createElement('button');
    outer.appendChild(inner);
    expect(scopeStackFor(inner)).toEqual(['global', 'view', 'chart']);
    expect(scopeStackFor(null)).toEqual(['global', 'view']);
  });

  // P0-4 carry-forwards (Ruling 88), Task 6.
  it('KeyboardEvent " " and the Go binding name "Space" are the same key', () => {
    expect(parseBinding('Ctrl+Space')?.key).toBe('Space');
    expect(bindingMatches('Ctrl+Space', ev(' ', 'Space', { ctrlKey: true }))).toBe(true);
    expect(bindingFromEvent(ev(' ', 'Space', { ctrlKey: true }))).toBe('Ctrl+Space');
  });
  it('an unbound action ("") never matches and never shadows another action', () => {
    expect(bindingMatches('', ev('a', 'KeyA', { altKey: true }))).toBe(false);
    const unbound = shortcutFixture.filter((a) => a.binding === '');
    expect(unbound.map((a) => a.id)).toEqual(['scalper.buy', 'scalper.sell', 'scalper.exitAll', 'scalper.cancelAll']);
    const withUnbound = [...unbound, ...actions];
    expect(resolveAction(withUnbound, ['global', 'view'], ev('t', 'KeyT', { altKey: true }))?.id).toBe('toasts.focus');
  });
  it('every default binding in the Go registry parses and round-trips through formatBinding', () => {
    for (const a of shortcutFixture.filter((x) => x.defaultBinding !== '')) {
      expect(parseBinding(a.defaultBinding), a.id).not.toBeNull();
      expect(formatBinding(a.defaultBinding), a.id).toBe(a.defaultBinding);
    }
  });
  it('the Go registry resolves Alt+T by scope: Focus toasts globally, Draw trend line on a chart', () => {
    const e = ev('t', 'KeyT', { altKey: true });
    expect(resolveAction(shortcutFixture, ['global', 'view'], e)?.label).toBe('Focus toasts');
    expect(resolveAction(shortcutFixture, ['global', 'view', 'chart'], e)?.label).toBe('Draw trend line');
  });
});
