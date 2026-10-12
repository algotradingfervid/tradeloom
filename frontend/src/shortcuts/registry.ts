import type { ShortcutAction, ShortcutScope } from '../api/backend';

export interface KeyBinding { key: string; ctrl: boolean; alt: boolean; shift: boolean; meta: boolean }
export interface KeyLike { key: string; code: string; ctrlKey: boolean; altKey: boolean; shiftKey: boolean; metaKey: boolean }

// Go (internal/settings ParseBinding) names the KeyboardEvent key " " as "Space"; the UI uses "Space" throughout.
const ALIASES: Record<string, string> = {
  up: 'ArrowUp', down: 'ArrowDown', left: 'ArrowLeft', right: 'ArrowRight',
  '↑': 'ArrowUp', '↓': 'ArrowDown', '←': 'ArrowLeft', '→': 'ArrowRight', esc: 'Escape', space: 'Space',
};

function normaliseKey(k: string): string {
  const alias = ALIASES[k.toLowerCase()];
  if (alias) return alias;
  if (/^f\d{1,2}$/i.test(k)) return k.toUpperCase();
  if (k.length === 1) return k.toUpperCase();
  return k[0].toUpperCase() + k.slice(1);
}

/** Parses a Go binding (`+`-joined Ctrl/Alt/Shift/Meta and a key). An empty binding (unbound action) is null. */
export function parseBinding(text: string): KeyBinding | null {
  if (!text) return null;
  const b: KeyBinding = { key: '', ctrl: false, alt: false, shift: false, meta: false };
  for (const raw of text.split('+')) {
    const part = raw.trim();
    if (!part) continue;
    const l = part.toLowerCase();
    if (l === 'ctrl' || l === 'control') b.ctrl = true;
    else if (l === 'alt') b.alt = true;
    else if (l === 'shift') b.shift = true;
    else if (l === 'meta' || l === 'super' || l === 'cmd') b.meta = true;
    else b.key = normaliseKey(part);
  }
  return b.key ? b : null;
}

export function formatBinding(text: string): string {
  const b = parseBinding(text);
  if (!b) return '';
  return [b.ctrl && 'Ctrl', b.alt && 'Alt', b.shift && 'Shift', b.meta && 'Meta', b.key].filter(Boolean).join('+');
}

/** Physical key for letters and digits (Alt+letter differs per layout), logical key otherwise. */
export function keyOfEvent(e: KeyLike): string {
  if (/^Key[A-Z]$/.test(e.code)) return e.code.slice(3);
  if (/^Digit\d$/.test(e.code)) return e.code.slice(5);
  if (e.key === ' ') return 'Space';
  return e.key.length === 1 ? e.key.toUpperCase() : e.key;
}

export function bindingMatches(binding: string, e: KeyLike): boolean {
  const b = parseBinding(binding);
  return b !== null && keyOfEvent(e) === b.key && e.ctrlKey === b.ctrl && e.altKey === b.alt && e.shiftKey === b.shift && e.metaKey === b.meta;
}

const MODIFIERS = new Set(['Control', 'Alt', 'Shift', 'Meta', 'AltGraph']);

export function bindingFromEvent(e: KeyLike): string | null {
  if (MODIFIERS.has(e.key)) return null;
  const parts: string[] = [];
  if (e.ctrlKey) parts.push('Ctrl');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  if (e.metaKey) parts.push('Meta');
  parts.push(keyOfEvent(e));
  return parts.join('+');
}

export function scopeStackFor(el: Element | null): ShortcutScope[] {
  const inner: ShortcutScope[] = [];
  for (let n: Element | null = el; n; n = n.parentElement) {
    const s = (n as HTMLElement).dataset?.shortcutScope as ShortcutScope | undefined;
    if (s) inner.unshift(s);
  }
  return ['global', 'view', ...inner];
}

/** Innermost scope first; an action without a handler is skipped so an outer one can take the key. */
export function resolveAction(
  actions: readonly ShortcutAction[],
  stack: readonly ShortcutScope[],
  e: KeyLike,
  hasHandler: (id: string) => boolean = () => true,
): ShortcutAction | undefined {
  for (let i = stack.length - 1; i >= 0; i--) {
    const hit = actions.find((a) => a.scope === stack[i] && hasHandler(a.id) && bindingMatches(a.binding, e));
    if (hit) return hit;
  }
  return undefined;
}

export function isTypingTarget(t: EventTarget | null): boolean {
  if (!(t instanceof HTMLElement)) return false;
  if (t.isContentEditable || t instanceof HTMLTextAreaElement || t instanceof HTMLSelectElement) return true;
  return t instanceof HTMLInputElement && !['checkbox', 'radio', 'button', 'submit', 'reset'].includes(t.type);
}
