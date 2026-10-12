import type { DragEvent, ReactNode } from 'react';
import { IconButton } from '../IconButton';
import { instrumentCopy, watchlistRowsCopy as copy } from '../../i18n/copy';
import { displayName, type LabelInstrument } from '../../instruments/naming';
import { useQuoteStale } from '../../market/quotes';
import { InstrumentLabel } from './InstrumentLabel';
import { PriceChange } from './PriceChange';
import './instrument.css';

export interface InstrumentRowProps {
  instrument: LabelInstrument;
  /** Price decimals; 2 for every in-scope tick size (0.05 / 0.10). */
  decimals?: number;
  selected: boolean;
  polled?: boolean;
  onSelect(): void;
  onChart?: () => void;
  onRemove?: () => void;
  onFocusRow?: () => void;
  dragHandlers?: {
    draggable: true;
    onDragStart(e: DragEvent<HTMLTableRowElement>): void;
    onDragOver(e: DragEvent<HTMLTableRowElement>): void;
    onDrop(e: DragEvent<HTMLTableRowElement>): void;
  };
  /** Extra trailing cell content (B/S in P4; nothing in P0). */
  trailing?: ReactNode;
}

/**
 * FR-WATCH-005 standard instrument row. Hover-only actions are also revealed on keyboard focus (NFR-11). The polled
 * and stale hints are words, not colour alone: "polled" when Go streams the token through its 5 s poller, "last
 * known" while the quote is a previous run's snapshot (Ruling 93; PriceChange announces the same to screen readers).
 */
export function InstrumentRow(p: InstrumentRowProps) {
  const name = displayName(p.instrument);
  const stale = useQuoteStale(p.instrument.token);
  return (
    <tr aria-selected={p.selected} aria-label={name} tabIndex={0} data-token={p.instrument.token}
      className={`instrument-row${p.selected ? ' is-selected' : ''}`} onFocus={p.onFocusRow}
      onKeyDown={(e) => { if (e.target === e.currentTarget && e.key === 'Enter') p.onSelect(); }} {...p.dragHandlers}>
      <td>
        <button type="button" className="instrument-row__symbol" onClick={p.onSelect}>
          <span className="instrument-row__avatar" aria-hidden="true">{name.charAt(0)}</span>
          <InstrumentLabel instrument={p.instrument} />
        </button>
      </td>
      <td className="instrument-row__price">
        <PriceChange token={p.instrument.token} decimals={p.decimals ?? 2} />
        {p.polled && <span className="instrument-row__hint" title={copy.polledHint}>{copy.polled}</span>}
        {stale && <span className="instrument-row__hint" title={instrumentCopy.staleQuote} aria-hidden="true">{copy.stale}</span>}
      </td>
      <td className="instrument-row__actions">
        {p.onChart && <IconButton label={copy.showOnChart(name)} icon="chevronRight" onClick={p.onChart} />}
        {p.onRemove && <IconButton label={copy.remove(name)} icon="close" onClick={p.onRemove} />}
        {p.trailing}
      </td>
    </tr>
  );
}
