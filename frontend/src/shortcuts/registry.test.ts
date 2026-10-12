import { describe, expect, it } from 'vitest';
import type { ShortcutAction } from '../api/backend';
import { shortcutFixture } from '../api/mock/fixtures';
import { bindingFromEvent, bindingMatches, formatBinding, isTypingTarget, parseBinding, resolveAction, scopeStackFor } from './registry';

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

  // Ruling 94 (Task 6 fix round 1).
  it('I1: punctuation keys match by physical key, so Shift+/ fires although e.key is "?"', () => {
    expect(bindingMatches('Shift+/', ev('?', 'Slash', { shiftKey: true }))).toBe(true);
    expect(bindingMatches('Ctrl+Shift+-', ev('_', 'Minus', { ctrlKey: true, shiftKey: true }))).toBe(true);
    expect(bindingMatches('Alt+;', ev('…', 'Semicolon', { altKey: true }))).toBe(true);
    expect(bindingFromEvent(ev('?', 'Slash', { ctrlKey: true, shiftKey: true }))).toBe('Ctrl+Shift+/');
    expect(bindingFromEvent(ev('"', 'Quote', { altKey: true, shiftKey: true }))).toBe("Alt+Shift+'");
  });
  it('I1: bindingFromEvent only produces bindings Go ParseBinding accepts, in Go canonical form', () => {
    const events = [
      ev('?', 'Slash', { ctrlKey: true, shiftKey: true }), ev('~', 'Backquote', { altKey: true }), ev('+', 'Equal', { ctrlKey: true, shiftKey: true }),
      ev('{', 'BracketLeft', { altKey: true, shiftKey: true }), ev('}', 'BracketRight', { altKey: true }), ev('|', 'Backslash', { ctrlKey: true, shiftKey: true }),
      ev(':', 'Semicolon', { altKey: true, shiftKey: true }), ev('<', 'Comma', { altKey: true, shiftKey: true }), ev('>', 'Period', { ctrlKey: true, shiftKey: true }),
      ev('†', 'KeyT', { altKey: true }), ev('¡', 'Digit1', { altKey: true }), ev(' ', 'Space', { ctrlKey: true }), ev('F11', 'F11'),
      ev('ArrowUp', 'ArrowUp', { altKey: true }), ev('PageDown', 'PageDown', { ctrlKey: true }), ev('Escape', 'Escape', { shiftKey: true }),
    ];
    for (const e of events) {
      const b = bindingFromEvent(e);
      expect(b, e.code).not.toBeNull();
      expect(goParseBinding(b as string), `${e.code} → ${b}`).toBe(b);
    }
  });
  it('M6: isTypingTarget covers text fields, contenteditable and select, but not checkboxes or buttons', () => {
    const text = document.createElement('input');
    const box = document.createElement('input');
    box.type = 'checkbox';
    const editor = document.createElement('div');
    editor.setAttribute('contenteditable', 'true');
    const inEditor = document.createElement('span');
    editor.appendChild(inEditor);
    const off = document.createElement('div');
    off.setAttribute('contenteditable', 'false');
    expect(isTypingTarget(text)).toBe(true);
    expect(isTypingTarget(document.createElement('textarea'))).toBe(true);
    expect(isTypingTarget(document.createElement('select'))).toBe(true);
    expect(isTypingTarget(editor)).toBe(true);
    expect(isTypingTarget(inEditor)).toBe(true);
    expect(isTypingTarget(off)).toBe(false);
    expect(isTypingTarget(box)).toBe(false);
    expect(isTypingTarget(document.createElement('button'))).toBe(false);
    expect(isTypingTarget(null)).toBe(false);
  });
});

/**
 * Mirror of Go internal/settings ParseBinding + Binding.String (shortcuts.go: namedKeys, punctuationKeys,
 * normaliseKey, ParseBinding, String): returns the canonical form, or null where Go would refuse the binding.
 */
function goParseBinding(s: string): string | null {
  const named = new Map([['space', ' '], ...['Escape', 'Enter', 'Tab', 'Backspace', 'Delete', 'Insert', 'Home', 'End', 'PageUp', 'PageDown', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight'].map((k): [string, string] => [k.toLowerCase(), k])]);
  const norm = (p: string): string | null => {
    const n = named.get(p.toLowerCase());
    if (n !== undefined) return n;
    if (p.length === 1) {
      if (/[a-z]/.test(p)) return p.toUpperCase();
      if (/[A-Z0-9]/.test(p) || "`-=[]\\;',./".includes(p)) return p;
      return null;
    }
    const m = /^[Ff]([1-9]\d?)$/.exec(p);
    return m && Number(m[1]) <= 24 ? `F${m[1]}` : null;
  };
  if (s.trim() === '') return null;
  const mods = { ctrl: false, alt: false, shift: false, meta: false };
  let key = '';
  const parts = s.trim().split('+');
  for (const [i, raw] of parts.entries()) {
    const p = raw.trim();
    if (p === '') return null;
    const last = i === parts.length - 1;
    const l = p.toLowerCase();
    const flag = l === 'ctrl' || l === 'control' ? 'ctrl' : l === 'alt' ? 'alt' : l === 'shift' ? 'shift' : ['meta', 'super', 'cmd'].includes(l) ? 'meta' : null;
    if (flag) {
      if (last || mods[flag]) return null;
      mods[flag] = true;
      continue;
    }
    if (!last) return null;
    const k = norm(p);
    if (k === null) return null;
    key = k;
  }
  if (key === '') return null;
  return [mods.ctrl && 'Ctrl', mods.alt && 'Alt', mods.shift && 'Shift', mods.meta && 'Meta', key === ' ' ? 'Space' : key].filter(Boolean).join('+');
}
