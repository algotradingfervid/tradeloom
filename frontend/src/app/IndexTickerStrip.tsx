import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { instrumentCopy, tickerCopy as copy } from '../i18n/copy';
import { formatPercent, formatPrice, formatSigned, toneOf } from '../i18n/format';
import type { Instrument } from '../instruments/types';
import { displayName } from '../instruments/naming';
import { useSubscription } from '../market/hooks';
import { quoteChange, useQuote, useQuoteStale } from '../market/quotes';
import { useResolvedInstrument } from '../store/instrumentsStore';
import './tickerStrip.css';

export const INDEX_PILLS = ['NSE:NIFTY 50', 'NSE:NIFTY BANK', 'BSE:SENSEX'] as const;
export const NARROW_BELOW_PX = 1440;

const ARROW = { up: '▲', down: '▼', flat: '' } as const;

function useWideWindow(): boolean {
  const [wide, setWide] = useState(() => window.innerWidth >= NARROW_BELOW_PX);
  useEffect(() => {
    const on = () => setWide(window.innerWidth >= NARROW_BELOW_PX);
    window.addEventListener('resize', on);
    return () => window.removeEventListener('resize', on);
  }, []);
  return wide;
}

/**
 * FR-SHELL-004: NIFTY 50, NIFTY BANK and SENSEX with live last price and change, one merged `widget` subscription.
 * A pill shows its index on the Home chart. `onShow` is P0-5 Task 10's showOnDashboard, wired in when that store
 * lands. Indices are not tradable: the pills are the only controls. Prices change without a live region, so a screen
 * reader hears them only when it reads a pill.
 */
export function IndexTickerStrip({ onShow }: { onShow?: (identifier: string) => void }) {
  const navigate = useNavigate();
  const wide = useWideWindow();
  // INDEX_PILLS is fixed, so the hook order is stable. useResolvedInstrument re-resolves after a reset (Ruling 107).
  const instruments = [
    useResolvedInstrument(INDEX_PILLS[0]).instrument,
    useResolvedInstrument(INDEX_PILLS[1]).instrument,
    useResolvedInstrument(INDEX_PILLS[2]).instrument,
  ];

  useSubscription('index-ticker-strip', 'widget', instruments.flatMap((i) => (i ? [i.token] : [])));

  return (
    <div className="ticker-strip" role="group" aria-label={copy.strip}>
      {INDEX_PILLS.map((id, k) => (
        <TickerPill key={id} identifier={id} instrument={instruments[k]} wide={wide}
          onOpen={() => { onShow?.(id); navigate('/home'); }} />
      ))}
    </div>
  );
}

function TickerPill({ identifier, instrument, wide, onOpen }: { identifier: string; instrument?: Instrument; wide: boolean; onOpen(): void }) {
  const q = useQuote(instrument?.token);
  const stale = useQuoteStale(instrument?.token);
  const ch = quoteChange(q);
  // PriceChange semantics: the tone follows the numbers shown (the percent alone when narrow); zero is flat.
  const dir = ch ? toneOf(wide ? ch.abs : ch.pct) : 'flat';
  // Until the identifier resolves, the symbol part stands in; the resolved name always comes from displayName().
  const label = instrument ? displayName(instrument) : identifier.slice(identifier.indexOf(':') + 1);
  const change = ch ? `${wide ? `${formatSigned(ch.abs)} ` : ''}(${formatPercent(ch.pct)})` : '';
  const name = !q
    ? `${label}, ${copy.noPrice}`
    : [label, formatPrice(q.ltp), ch && `${copy[dir]} ${change}`].filter(Boolean).join(', ') + (stale ? `. ${instrumentCopy.staleQuote}` : '');
  const show = copy.show(label);
  return (
    <button
      type="button"
      className={`ticker-pill ticker-pill--${q ? dir : 'empty'}${stale ? ' ticker-pill--stale' : ''}`}
      aria-label={name}
      title={stale ? `${instrumentCopy.staleQuote}. ${show}` : show}
      data-stale={stale ? 'true' : undefined}
      onClick={onOpen}
    >
      <span className="ticker-pill__name" {...(instrument ? { 'data-instrument-name': '', 'data-instrument-token': instrument.token } : {})}>{label}</span>
      <span className="ticker-pill__ltp">{q ? formatPrice(q.ltp) : '—'}</span>
      {ch && (
        <span className="ticker-pill__change">
          {dir !== 'flat' && <span aria-hidden="true">{ARROW[dir]}</span>}
          {change}
        </span>
      )}
    </button>
  );
}
