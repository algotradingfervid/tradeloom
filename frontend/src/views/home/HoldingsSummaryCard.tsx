import { useId, useMemo, type ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { BackgroundErrorNotice } from '../../components/BackgroundErrorNotice';
import { Button } from '../../components/Button';
import { Skeleton } from '../../components/Skeleton';
import { Signed } from '../../components/Values';
import { holdingsCardCopy as copy, listCopy } from '../../i18n/copy';
import { formatMoney, toneOf } from '../../i18n/format';
import { holdingRow, liveLtp, summarizeHoldings } from '../../portfolio/calc';
import { usePortfolioResource } from '../../store/portfolio';
import { useTicksStore } from '../../store/ticks';
import './home.css';

export const HOLDINGS_REFRESH_MS = 300_000;

/**
 * FR-HOME-002. Totals are Go's `HoldingsSummary` (integer paise, every row incl. out-of-scope ones, P0-3 Ruling 38)
 * plus each row's live-LTP difference in paise; quotes persisted by an earlier run are ignored (Ruling 93: liveLtp → isStaleQuote/runStartedMs).
 */
export function HoldingsSummaryCard() {
  const titleId = useId();
  // Reads the Go cache on mount and every 5 minutes; fills arrive in between as "holdings.changed" snapshots
  // (order.update is a notification only, P0-3 Ruling 71).
  const { resource, reload, retryInBackground } = usePortfolioResource('holdings', { pollMs: HOLDINGS_REFRESH_MS });
  const latest = useTicksStore((s) => s.latest);
  const rows = useMemo(
    () => resource.data.rows.map((h) => holdingRow(h, liveLtp(latest[h.token]))),
    [resource.data, latest],
  );
  const s = summarizeHoldings(resource.data.summary, rows);
  // A failed re-read while figures are shown keeps them with a notice (Task 10 M8, Ruling 105); the alert is only for
  // a failure with no data held.
  const hasData = resource.data.asOfMs > 0;

  let body: ReactNode;
  if (resource.status === 'idle') {
    body = <Skeleton rows={3} label={copy.loading} />;
  } else if (resource.status === 'error' && !hasData) {
    body = (
      <div className="card__error" role="alert">
        <p>{copy.error(resource.error ?? '')}</p>
        <Button onClick={() => void reload()}>{listCopy.retry}</Button>
      </div>
    );
  } else if (s.empty || rows.length === 0) {
    body = <Link to="/holdings" className="holdings-card__body" title={copy.open}>{copy.empty}</Link>;
  } else {
    const pnlTone = toneOf(s.pnl);
    body = (
      <Link to="/holdings" className="holdings-card__body" title={copy.open}>
        <dl className="holdings-card__grid">
          <div><dt>{copy.currentValue}</dt><dd>{formatMoney(s.currentValue)}</dd></div>
          <div><dt>{copy.investment}</dt><dd>{formatMoney(s.investment)}</dd></div>
          <div>
            <dt>{copy.pnl}</dt>
            <dd className={`tone-${pnlTone}`}>
              {pnlTone !== 'flat' && <span aria-hidden="true">{pnlTone === 'up' ? '▲ ' : '▼ '}</span>}
              <Signed value={s.pnl} kind="money" /> (<Signed value={s.pnlPct} kind="percent" />)
            </dd>
          </div>
          <div><dt>{copy.dayChange}</dt><dd><Signed value={s.dayChange} kind="money" /></dd></div>
        </dl>
      </Link>
    );
  }

  return (
    <section className="card holdings-card" aria-labelledby={titleId}>
      <h2 id={titleId} className="card__title">{copy.title}</h2>
      {body}
      {/* A failed background re-read keeps the shown figures (Task 10 M8, Ruling 103). */}
      <BackgroundErrorNotice error={resource.backgroundError} seq={resource.backgroundErrorSeq} message={copy.staleNotice} onRetry={retryInBackground} className="card__notice" />
    </section>
  );
}
