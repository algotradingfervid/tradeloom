import { instrumentCopy as copy } from '../../i18n/copy';
import { formatPercent, formatPrice, formatSigned, toneOf } from '../../i18n/format';
import { quoteChange, useQuote, useQuoteStale } from '../../market/quotes';
import './instrument.css';

/**
 * Last price and change vs the previous close for one token (prices are rupees, as in TickDTO). The change is always
 * signed in text; the tone class only adds colour. No quote → "—"; no previous close → the price alone (never a fake
 * +0.00%). A quote from a previous run (persisted Watch snapshot) is shown with a stale cue until a fresh tick
 * arrives (Ruling 93).
 */
export function PriceChange({ token, decimals = 2, showAbsolute = false }: { token: number; decimals?: number; showAbsolute?: boolean }) {
  const q = useQuote(token);
  const stale = useQuoteStale(token);
  if (!q) return <span className="price-change price-change--empty">—</span>;
  const ch = quoteChange(q);
  const dir = ch ? toneOf(ch.abs) : 'flat';
  return (
    <span
      className={`price-change${stale ? ' price-change--stale' : ''}`}
      data-direction={dir}
      data-stale={stale ? 'true' : undefined}
      title={stale ? copy.staleQuote : undefined}
    >
      <span className="price-change__ltp">{formatPrice(q.ltp, decimals)}</span>
      {ch && (
        <span className={`price-change__delta tone-${dir}`}>
          {showAbsolute ? `${formatSigned(ch.abs, decimals)} ` : ''}({formatPercent(ch.pct)})
        </span>
      )}
      {stale && <span className="visually-hidden">{copy.staleQuote}</span>}
    </span>
  );
}
