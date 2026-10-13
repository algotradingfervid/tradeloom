import { goMarshalString, goQuote, goStringSlice } from './go';

/**
 * The mock Settings.Set checks (Ruling 87): a port of Go `service.SetSetting` (internal/service/settings.go) over the
 * `settings.Register`ed keys (internal/settings/keys.go, keys_ui.go, registry.go init, internal/service/watchlists.go;
 * the kite.* keys are refused earlier by the caller). Codes and reasons are Go's exactly:
 *   - not valid JSON → invalid_setting "The value is not valid JSON."
 *   - unregistered name → unknown_setting "Unknown setting <name>."
 *   - wrong JSON type → invalid_setting "settings: <name>: json: cannot unmarshal … into Go value of type …"
 *   - Validate rejects → invalid_setting "settings: <name>: <validator message>"
 * As in Go, JSON null unmarshals to the zero value, which is then validated and stored.
 */

export class SettingRefusal extends Error {
  constructor(readonly code: string, readonly reason: string) {
    super(reason);
  }
}

type Kind = 'string' | 'bool' | 'int';
type Validate = (v: never) => string | null;
interface KeyDef { kind: Kind; validate?: Validate }

/** settings.OneOf */
const oneOf = (...allowed: string[]) => (v: string) => (allowed.includes(v) ? null : `${goQuote(v)} is not one of ${goStringSlice(allowed)}`);
/** settings.IntRange */
const intRange = (lo: number, hi: number) => (v: number) => (v < lo || v > hi ? `${v} is outside ${lo}…${hi}` : null);
/** settings.AbsPathOrEmpty (filepath.IsAbs on Unix). */
const absPathOrEmpty = (v: string) => (v !== '' && !v.startsWith('/') ? 'must be an absolute path' : null);

/** settings.ValidateRoute */
function validateRoute(v: string): string | null {
  if (!v.startsWith('/') || /[ \t\r\n]/.test(v)) return 'must be an in-app route starting with /';
  if (v.startsWith('//')) return 'must not start with //';
  if (v.split('/').includes('..')) return 'must not contain .. segments';
  return null;
}

const isLeap = (y: number) => (y % 4 === 0 && y % 100 !== 0) || y % 400 === 0;

/** settings.ValidateDateOrEmpty: "" or Go time.Parse("2006-01-02", v). */
function validateDateOrEmpty(v: string): string | null {
  if (v === '') return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(v);
  const bad = 'must be a date formatted YYYY-MM-DD';
  if (!m) return bad;
  const [y, mo, d] = [Number(m[1]), Number(m[2]), Number(m[3])];
  const days = [31, isLeap(y) ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
  return mo < 1 || mo > 12 || d < 1 || d > days[mo - 1] ? bad : null;
}

/** settings.KiteIntervals */
const KITE_INTERVALS = ['minute', '3minute', '5minute', '10minute', '15minute', '30minute', '60minute', 'day'];

const REGISTRY: Record<string, KeyDef> = {
  'log.level': { kind: 'string', validate: oneOf('info', 'debug') },
  'backup.schedule.enabled': { kind: 'bool' },
  'backup.schedule.folder': { kind: 'string', validate: absPathOrEmpty },
  'backup.schedule.keep': { kind: 'int', validate: intRange(1, 365) },
  'backup.include_candles': { kind: 'bool' },
  'appearance.theme': { kind: 'string', validate: oneOf('light', 'dark') },
  'appearance.follow_system': { kind: 'bool' },
  'appearance.sidebar_collapsed': { kind: 'bool' },
  'ui.last_route': { kind: 'string', validate: validateRoute },
  'chart.default_timeframe': { kind: 'string', validate: oneOf(...KITE_INTERVALS) },
  'calendar.stale_toast_day': { kind: 'string', validate: validateDateOrEmpty },
  'watchlist.active': { kind: 'string' },
};

const INT64_MIN = -(2n ** 63n);
const INT64_MAX = 2n ** 63n - 1n;

/** encoding/json's UnmarshalTypeError.Value for a top-level value that does not fit the Go type. */
function jsonTypeName(v: unknown): string {
  if (Array.isArray(v)) return 'array';
  if (typeof v === 'boolean') return 'bool';
  return typeof v; // 'string' | 'number' | 'object'
}

/** Go json.Unmarshal(raw, &v) for v of kind; returns the Go value or the error text. */
function unmarshal(kind: Kind, raw: string, parsed: unknown): { value: string | boolean | number } | { err: string } {
  const typeErr = (what: string) => ({ err: `json: cannot unmarshal ${what} into Go value of type ${kind}` });
  if (parsed === null) return { value: kind === 'string' ? '' : kind === 'bool' ? false : 0 };
  switch (kind) {
    case 'string':
      return typeof parsed === 'string' ? { value: parsed } : typeErr(jsonTypeName(parsed));
    case 'bool':
      return typeof parsed === 'boolean' ? { value: parsed } : typeErr(jsonTypeName(parsed));
    case 'int': {
      if (typeof parsed !== 'number') return typeErr(jsonTypeName(parsed));
      const lit = raw.trim(); // the number literal as written (strconv.ParseInt(item, 10, 64))
      if (!/^-?\d+$/.test(lit) || BigInt(lit) < INT64_MIN || BigInt(lit) > INT64_MAX) return typeErr(`number ${lit}`);
      return { value: Number(lit) };
    }
  }
}

/** Go `service.SetSetting` after the credential-key check: returns the canonical stored JSON or throws SettingRefusal. */
export function checkSetting(name: string, valueJSON: string): { valueJson: string; value: unknown } {
  let parsed: unknown;
  try {
    parsed = JSON.parse(valueJSON);
  } catch {
    throw new SettingRefusal('invalid_setting', 'The value is not valid JSON.');
  }
  const def = REGISTRY[name];
  if (!def) throw new SettingRefusal('unknown_setting', `Unknown setting ${name}.`);
  const got = unmarshal(def.kind, valueJSON, parsed);
  if ('err' in got) throw new SettingRefusal('invalid_setting', `settings: ${name}: ${got.err}`);
  const msg = (def.validate as ((v: unknown) => string | null) | undefined)?.(got.value);
  if (msg) throw new SettingRefusal('invalid_setting', `settings: ${name}: ${msg}`);
  // settings.Set stores json.Marshal(v).
  const valueJson = typeof got.value === 'string' ? goMarshalString(got.value) : JSON.stringify(got.value);
  return { valueJson, value: got.value };
}
