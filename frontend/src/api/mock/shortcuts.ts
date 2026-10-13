import type { ShortcutAction, ShortcutScope } from '../backend';
import { byteLen, goQuote, goTrimSpace } from './go';

/**
 * The mock Settings.Rebind checks (Ruling 87): a port of Go `service.Rebind` (internal/service/shortcuts.go) and of
 * `settings.ParseBinding`, `ValidateBinding` and `Conflicts` (internal/settings/shortcuts.go), same codes and reasons.
 */

export class ShortcutRefusal extends Error {
  constructor(readonly code: string, readonly reason: string) {
    super(reason);
  }
}

class BindingError extends Error {}

interface Binding { ctrl: boolean; alt: boolean; shift: boolean; meta: boolean; key: string }

/** Go service.maxIDBytes */
const MAX_ID_BYTES = 64;

/** settings.namedKeys: lower-cased spelling → KeyboardEvent.key spelling ("Space" is the only alias, of " "). */
const NAMED_KEYS: Record<string, string> = { space: ' ' };
for (const k of ['Escape', 'Enter', 'Tab', 'Backspace', 'Delete', 'Insert', 'Home', 'End', 'PageUp', 'PageDown', 'ArrowUp', 'ArrowDown', 'ArrowLeft', 'ArrowRight']) {
  NAMED_KEYS[k.toLowerCase()] = k;
}

const PUNCTUATION_KEYS = "`-=[]\\;',./";

/** settings.normaliseKey */
function normaliseKey(p: string): string | null {
  const named = NAMED_KEYS[p.toLowerCase()];
  if (named !== undefined) return named;
  if (byteLen(p) === 1) {
    if (p >= 'a' && p <= 'z') return p.toUpperCase();
    if ((p >= 'A' && p <= 'Z') || (p >= '0' && p <= '9') || PUNCTUATION_KEYS.includes(p)) return p;
    return null;
  }
  if ((p[0] === 'F' || p[0] === 'f') && byteLen(p) <= 3) {
    const digits = p.slice(1);
    if (!/^[0-9]+$/.test(digits)) return null;
    const n = Number(digits);
    if (n >= 1 && n <= 24 && digits[0] !== '0') return `F${digits}`;
  }
  return null;
}

/** settings.ParseBinding */
function parseBinding(s: string): Binding {
  const b: Binding = { ctrl: false, alt: false, shift: false, meta: false, key: '' };
  if (goTrimSpace(s) === '') throw new BindingError('binding is empty');
  const parts = goTrimSpace(s).split('+');
  parts.forEach((raw, i) => {
    const p = goTrimSpace(raw);
    if (p === '') throw new BindingError(`binding ${goQuote(s)} has an empty part`);
    const last = i === parts.length - 1;
    let flag: 'ctrl' | 'alt' | 'shift' | 'meta' | null = null;
    switch (p.toLowerCase()) {
      case 'ctrl': case 'control': flag = 'ctrl'; break;
      case 'alt': flag = 'alt'; break;
      case 'shift': flag = 'shift'; break;
      case 'meta': case 'super': case 'cmd': flag = 'meta'; break;
    }
    if (flag) {
      if (last || b[flag]) throw new BindingError(`binding ${goQuote(s)}: ${goQuote(p)} is a modifier, not a key`);
      b[flag] = true;
      return;
    }
    if (!last) throw new BindingError(`binding ${goQuote(s)}: ${goQuote(p)} must be the last part`);
    const k = normaliseKey(p);
    if (k === null) throw new BindingError(`binding ${goQuote(s)}: ${goQuote(p)} is not a supported key`);
    b.key = k;
  });
  if (b.key === '') throw new BindingError(`binding ${goQuote(s)} has no key`);
  return b;
}

/** settings.Binding.String: Ctrl+Alt+Shift+Meta order, then the key. */
function bindingString(b: Binding): string {
  const parts: string[] = [];
  if (b.ctrl) parts.push('Ctrl');
  if (b.alt) parts.push('Alt');
  if (b.shift) parts.push('Shift');
  if (b.meta) parts.push('Meta');
  parts.push(b.key === ' ' ? 'Space' : b.key);
  return parts.join('+');
}

/** settings.reservedBindings */
const RESERVED = new Set(['Ctrl+C', 'Ctrl+V', 'Ctrl+X', 'Ctrl+A', 'Ctrl+Q', 'Ctrl+W', 'Alt+F4', 'F5', 'F12']);

/** settings.ValidateBinding: the canonical form, or throws BindingError. */
function validateBinding(scope: ShortcutScope, s: string): string {
  if (!['global', 'view', 'chart', 'builder'].includes(scope)) throw new BindingError(`unknown shortcut scope ${goQuote(scope)}`);
  const b = parseBinding(s);
  const canon = bindingString(b);
  if (RESERVED.has(canon)) throw new BindingError(`${canon} is reserved and cannot be used`);
  if (scope === 'global' || scope === 'view') {
    const fn = b.key.length >= 2 && b.key[0] === 'F' && b.key[1] >= '0' && b.key[1] <= '9';
    if (!fn && !b.ctrl && !b.alt && !b.meta) throw new BindingError(`${canon} needs Ctrl, Alt or Meta in the ${scope} scope`);
  }
  return canon;
}

/** settings.Conflicts: another action using binding in the same scope as self or in (or against) the global scope. */
function conflicts(actions: ShortcutAction[], self: ShortcutAction, binding: string): ShortcutAction | undefined {
  if (binding === '') return undefined;
  return actions.find((a) => {
    if (a.id === self.id || a.binding !== binding) return false;
    if (a.scope !== self.scope && a.scope !== 'global' && self.scope !== 'global') return false;
    return !(binding === self.defaultBinding && a.binding === a.defaultBinding); // shipped overlap
  });
}

/**
 * Go `service.Rebind` up to the save: returns the action to update and the canonical binding, or throws
 * ShortcutRefusal (invalid / unknown_action / conflict).
 */
export function checkRebind(actions: ShortcutAction[], actionId: string, binding: string): { target: ShortcutAction; canon: string } {
  if (byteLen(actionId) > MAX_ID_BYTES || byteLen(binding) > MAX_ID_BYTES) {
    throw new ShortcutRefusal('invalid', 'Shortcut action or binding is too long.');
  }
  const target = actions.find((a) => a.id === actionId);
  if (!target) throw new ShortcutRefusal('unknown_action', 'Unknown shortcut action.');
  let canon: string;
  try {
    canon = validateBinding(target.scope, binding);
  } catch (e) {
    if (e instanceof BindingError) throw new ShortcutRefusal('invalid', e.message);
    throw e;
  }
  const other = conflicts(actions, target, canon);
  if (other) throw new ShortcutRefusal('conflict', other.label);
  return { target, canon };
}
