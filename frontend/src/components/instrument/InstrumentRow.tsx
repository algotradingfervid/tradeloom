import { useId, type DragEvent, type KeyboardEvent, type ReactNode } from 'react';
import { IconButton } from '../IconButton';
import { instrumentCopy as copy } from '../../i18n/copy';
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
    onDragEnd?(e: DragEvent<HTMLTableRowElement>): void;
  };
  /** Extra trailing cell content (B/S in P4; nothing in P0). */
  trailing?: ReactNode;
  /**
   * Roving tabindex (Ruling 115 I3): false takes the row and its action buttons out of the tab order; the list's
   * arrow keys reach it instead. Defaults to true (a standalone row is one tab stop plus its actions).
   */
  tabbable?: boolean;
  /** Arrow/Home/End handling of the owning list. */
  onRowKeyDown?: (e: KeyboardEvent<HTMLTableRowElement>) => void;
  /** aria-keyshortcuts for the row (e.g. the reorder bindings). */
  keyShortcuts?: string;
}

/**
 * FR-WATCH-005 standard instrument row. Hover-only actions are also revealed on keyboard focus (NFR-11); Enter on the
 * row selects it, so the symbol button stays out of the tab order. The polled and stale hints are words, not colour
 * alone: "polled" (its explanation is the row's description, so it is not hover-only) and "last known" while the
 * quote is a previous run's snapshot (Ruling 93; PriceChange announces the same to screen readers).
 */
export function InstrumentRow(p: InstrumentRowProps) {
  const name = displayName(p.instrument);
  const stale = useQuoteStale(p.instrument.token);
  const hintId = useId();
  const tabbable = p.tabbable ?? true;
  const actionTab = tabbable ? undefined : -1;
  return (
    <tr aria-selected={p.selected} aria-label={name} aria-describedby={p.polled ? hintId : undefined}
      aria-keyshortcuts={p.keyShortcuts || undefined} tabIndex={tabbable ? 0 : -1} data-token={p.instrument.token}
      className={`instrument-row${p.selected ? ' is-selected' : ''}`} onFocus={p.onFocusRow}
      onKeyDown={(e) => {
        if (e.target === e.currentTarget && e.key === 'Enter') p.onSelect();
        else p.onRowKeyDown?.(e);
      }} {...p.dragHandlers}>
      <td>
        <button type="button" className="instrument-row__symbol" tabIndex={-1} onClick={p.onSelect}>
          <span className="instrument-row__avatar" aria-hidden="true">{name.charAt(0)}</span>
          <InstrumentLabel instrument={p.instrument} />
        </button>
      </td>
      <td className="instrument-row__price">
        <PriceChange token={p.instrument.token} decimals={p.decimals ?? 2} />
        {p.polled && (
          <>
            <span className="instrument-row__hint" title={copy.polledHint}>{copy.polled}</span>
            <span id={hintId} className="visually-hidden">{copy.polledHint}</span>
          </>
        )}
        {stale && <span className="instrument-row__hint" title={copy.staleQuote} aria-hidden="true">{copy.stale}</span>}
      </td>
      <td className="instrument-row__actions">
        {p.onChart && <IconButton label={copy.showOnChart(name)} icon="chevronRight" tabIndex={actionTab} onClick={p.onChart} />}
        {p.onRemove && <IconButton label={copy.remove(name)} icon="close" tabIndex={actionTab} onClick={p.onRemove} />}
        {p.trailing}
      </td>
    </tr>
  );
}
