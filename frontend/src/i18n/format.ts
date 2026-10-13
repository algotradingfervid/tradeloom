export const DASH = '—';

const TZ = 'Asia/Kolkata';
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAY_MS = 86_400_000;

type Num = number | null | undefined;

const numberFormats = new Map<number, Intl.NumberFormat>();
function grouping(digits: number): Intl.NumberFormat {
  let f = numberFormats.get(digits);
  if (!f) {
    f = new Intl.NumberFormat('en-IN', { minimumFractionDigits: digits, maximumFractionDigits: digits, useGrouping: true });
    numberFormats.set(digits, f);
  }
  return f;
}

function isNum(v: Num): v is number {
  return typeof v === 'number' && Number.isFinite(v);
}

/** Round to `digits` decimals and normalise -0 to 0 so "-0.00" can never be printed. */
function roundTo(v: number, digits: number): number {
  const r = Number(v.toFixed(digits));
  return r === 0 ? 0 : r;
}

function signed(v: number, digits: number, prefix: string): string {
  const r = roundTo(v, digits);
  return (r < 0 ? '-' : '+') + prefix + grouping(digits).format(Math.abs(r));
}

export function formatNumber(v: Num, digits = 2): string {
  return isNum(v) ? grouping(digits).format(roundTo(v, digits)) : DASH;
}

export function formatPrice(v: Num, tickDecimals = 2): string {
  return formatNumber(v, tickDecimals);
}

export function formatMoney(v: Num, digits = 2): string {
  if (!isNum(v)) return DASH;
  const r = roundTo(v, digits);
  return (r < 0 ? '-' : '') + '₹' + grouping(digits).format(Math.abs(r));
}

export function formatSignedMoney(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '₹') : DASH;
}

export function formatSigned(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '') : DASH;
}

export function formatPercent(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '') + '%' : DASH;
}

export function formatChange(abs: Num, pct: Num): string {
  if (!isNum(abs) || !isNum(pct)) return DASH;
  return `${formatSigned(abs)} (${formatPercent(pct)})`;
}

export function formatCompact(v: Num): string {
  if (!isNum(v)) return DASH;
  const a = Math.abs(v);
  const sign = v < 0 ? '-' : '';
  if (a >= 1e7 || roundTo(a / 1e5, 2) >= 100) return sign + grouping(2).format(roundTo(a / 1e7, 2)) + 'Cr';
  if (a >= 1e5 || Math.round(a) >= 1e5) return sign + grouping(2).format(roundTo(a / 1e5, 2)) + 'L';
  return formatNumber(v, 0);
}

export function toneOf(v: Num): 'up' | 'down' | 'flat' {
  if (!isNum(v)) return 'flat';
  const r = roundTo(v, 2);
  return r > 0 ? 'up' : r < 0 ? 'down' : 'flat';
}

const dateTimeParts = new Intl.DateTimeFormat('en-IN', {
  timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
});

interface IstParts { y: number; m: number; d: number; hh: string; mm: string; ss: string }

function istParts(ms: number): IstParts {
  const p: Record<string, string> = {};
  for (const part of dateTimeParts.formatToParts(new Date(ms))) p[part.type] = part.value;
  return { y: Number(p.year), m: Number(p.month), d: Number(p.day), hh: p.hour, mm: p.minute, ss: p.second };
}

const pad2 = (n: number) => String(n).padStart(2, '0');
const validTime = (ms: Num): ms is number => isNum(ms) && ms > 0;

export function formatDate(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${pad2(p.d)}-${pad2(p.m)}-${p.y}`;
}

export function formatDateLong(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${pad2(p.d)} ${MONTHS[p.m - 1]} ${p.y}`;
}

export function formatDateTimeLong(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${formatDateLong(ms)}, ${p.hh}:${p.mm}`;
}

export function formatDateTimeCompact(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${formatDate(ms)} ${p.hh}:${p.mm}:${p.ss}`;
}

export function formatTime(ms: Num, withSeconds = false): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return withSeconds ? `${p.hh}:${p.mm}:${p.ss}` : `${p.hh}:${p.mm}`;
}

export function istDayKey(ms: number): string {
  const p = istParts(ms);
  return `${p.y}-${pad2(p.m)}-${pad2(p.d)}`;
}

function istDayIndex(ms: number): number {
  const p = istParts(ms);
  return Date.UTC(p.y, p.m - 1, p.d) / DAY_MS;
}

/** "06:00 today", "06:00 tomorrow", otherwise the long style. `now` is app-clock time. */
export function formatExpiry(expiresAt: Num, now: number): string {
  if (!validTime(expiresAt)) return DASH;
  const delta = istDayIndex(expiresAt) - istDayIndex(now);
  const time = formatTime(expiresAt);
  if (delta === 0) return `${time} today`;
  if (delta === 1) return `${time} tomorrow`;
  return formatDateTimeLong(expiresAt);
}

const BYTE_UNITS = ['B', 'KB', 'MB', 'GB', 'TB'];

/** Binary units (1 KB = 1024 B): two decimals below 10, one decimal otherwise. */
export function formatBytes(v: Num): string {
  if (!isNum(v) || v < 0) return DASH;
  let n = v;
  let i = 0;
  while (n >= 1024 && i < BYTE_UNITS.length - 1) {
    n /= 1024;
    i += 1;
  }
  if (i === 0) return `${grouping(0).format(n)} B`;
  const digits = n < 10 ? 2 : 1;
  return `${grouping(digits).format(roundTo(n, digits))} ${BYTE_UNITS[i]}`;
}
