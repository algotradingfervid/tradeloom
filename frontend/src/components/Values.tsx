import { formatPercent, formatSigned, formatSignedMoney, toneOf } from '../i18n/format';

/** A signed value: the text always carries + or − (never colour alone); the tone class only adds colour. */
export function Signed({ value, kind = 'number' }: { value: number | null; kind?: 'money' | 'percent' | 'number' }) {
  const text = kind === 'money' ? formatSignedMoney(value) : kind === 'percent' ? formatPercent(value) : formatSigned(value);
  return <span className={`tone-${toneOf(value)}`}>{text}</span>;
}

/**
 * Symbol, exchange chip and an optional flag (e.g. "View only" for out-of-scope rows, P0-3 Ruling 38). Only the name
 * truncates; the chips never get clipped, and the full text (name plus flag) is the cell's `title` (Ruling 96 I2).
 * Use it in a column without `Column.title`, which would wrap the whole cell in a truncating span.
 */
export function InstrumentName({ name, exchange, flag, flagHint }: { name: string; exchange: string; flag?: string; flagHint?: string }) {
  return (
    <span className="instrument" title={flag ? `${name} · ${flag}` : name}>
      <span className="instrument__name">{name}</span>
      <span className="chip">{exchange}</span>
      {flag && <span className="chip chip--flag" aria-description={flagHint}>{flag}</span>}
    </span>
  );
}
