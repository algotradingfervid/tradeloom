import { formatPercent, formatSigned, formatSignedMoney, toneOf } from '../i18n/format';

/** A signed value: the text always carries + or − (never colour alone); the tone class only adds colour. */
export function Signed({ value, kind = 'number' }: { value: number | null; kind?: 'money' | 'percent' | 'number' }) {
  const text = kind === 'money' ? formatSignedMoney(value) : kind === 'percent' ? formatPercent(value) : formatSigned(value);
  return <span className={`tone-${toneOf(value)}`}>{text}</span>;
}

export function InstrumentName({ name, exchange }: { name: string; exchange: string }) {
  return (
    <>
      <span className="instrument__name">{name}</span>
      <span className="chip">{exchange}</span>
    </>
  );
}
