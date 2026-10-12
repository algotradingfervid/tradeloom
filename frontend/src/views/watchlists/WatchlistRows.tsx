import { useLayoutEffect, useRef, useState, type DragEvent, type FocusEvent, type MutableRefObject } from 'react';
import type { WatchlistItem } from '../../api/backend';
import { parseBackendError } from '../../api/errors';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { InstrumentRow, type InstrumentRowProps } from '../../components/instrument/InstrumentRow';
import { IconButton } from '../../components/IconButton';
import { watchlistRowsCopy as copy } from '../../i18n/copy';
import { displayName, labelOfItem, OutOfScopeError, type LabelInstrument } from '../../instruments/naming';
import { usePolled, useSubscription } from '../../market/hooks';
import { useShortcut } from '../../shortcuts/ShortcutProvider';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists, type Watchlist } from '../../store/watchlistsStore';
import './watchlists.css';

const reason = (e: unknown): string => (e instanceof Error ? e.message : parseBackendError(e).reason);
const reportError = (text: (r: string) => string) => (e: unknown) => {
  pushToast({ level: 'error', message: text(reason(e)) });
};

/**
 * Item → label through labelOfItem (Ruling 107). An item Go sends outside the NFR-15 scope keeps its raw exchange as
 * the tag, so InstrumentLabel shows its name with the "View only" flag instead of the panel throwing (P0-3 Ruling 38).
 * A missing item in Go's shape (no exchange, no symbol) has no label.
 */
function labelOf(item: WatchlistItem): LabelInstrument | null {
  try {
    return labelOfItem(item);
  } catch (e) {
    if (!(e instanceof OutOfScopeError) || !item.symbol) return null;
    return { token: item.token, tag: item.exchange, displayName: item.displayName, tradingsymbol: item.symbol, name: item.name };
  }
}

/** Registered only while a row has focus, so Alt+Up/Down stay free everywhere else (Ruling 94 M1). */
function ReorderShortcuts({ token, onMove }: { token: number; onMove(token: number, delta: -1 | 1): void }) {
  useShortcut('watchlist.moveUp', () => onMove(token, -1));
  useShortcut('watchlist.moveDown', () => onMove(token, 1));
  return null;
}

export function WatchlistRows({ list, onChartItem }: { list: Watchlist; onChartItem(item: WatchlistItem): void }) {
  // Missing items (token no longer in the master) cannot stream; everything else is the watchlist consumer's set.
  useSubscription('watchlist-panel', 'watchlist', list.items.filter((i) => !i.missing).map((i) => i.token));
  const selected = useWatchlists((s) => s.selectedToken);
  // Rows are tracked by token, never by index (P0-4 Ruling 84).
  const [focused, setFocused] = useState<number | null>(null);
  const dragged = useRef<number | null>(null);
  const body = useRef<HTMLTableSectionElement>(null);
  /** The token whose row keeps focus while its move is in flight (React may re-insert the focused <tr>). */
  const following = useRef<number | null>(null);

  useLayoutEffect(() => {
    const token = following.current;
    if (token === null || !body.current) return;
    const row = body.current.querySelector<HTMLElement>(`tr[data-token="${token}"]`);
    if (row && !row.contains(document.activeElement)) row.focus();
  });

  const move = (token: number, delta: -1 | 1) => {
    following.current = token;
    useWatchlists.getState().move(token, delta)
      .catch(reportError(copy.moveFailed))
      .finally(() => { if (following.current === token) following.current = null; });
  };

  const onFocus = (e: FocusEvent<HTMLTableSectionElement>) => {
    const row = (e.target as Element).closest<HTMLElement>('tr[data-token]');
    setFocused(row ? Number(row.dataset.token) : null);
  };
  const onBlur = (e: FocusEvent<HTMLTableSectionElement>) => {
    if (!(e.relatedTarget instanceof Node && e.currentTarget.contains(e.relatedTarget))) setFocused(null);
  };

  if (list.items.length === 0) return <p className="watchlist-empty">{copy.empty}</p>;

  return (
    <table className="watchlist-table" role="grid" aria-label={copy.tableLabel(list.name)}>
      <thead>
        <tr><th scope="col">{copy.symbolColumn}</th><th scope="col">{copy.priceColumn}</th><th scope="col"><span className="visually-hidden">{copy.actionsColumn}</span></th></tr>
      </thead>
      <tbody ref={body} onFocus={onFocus} onBlur={onBlur}>
        {list.items.map((item) => (
          <WatchlistRow key={item.token} item={item} listName={list.name} selected={selected === item.token}
            onChartItem={onChartItem} onFocusRow={() => setFocused(item.token)} dragged={dragged} />
        ))}
      </tbody>
      {focused !== null && list.items.some((i) => i.token === focused) && <ReorderShortcuts token={focused} onMove={move} />}
    </table>
  );
}

interface RowProps {
  item: WatchlistItem; listName: string; selected: boolean;
  onChartItem(item: WatchlistItem): void; onFocusRow(): void; dragged: MutableRefObject<number | null>;
}

function WatchlistRow({ item, listName, selected, onChartItem, onFocusRow, dragged }: RowProps) {
  const polled = usePolled(item.token);
  const label = labelOf(item);
  const name = label ? displayName(label) : copy.missingName;

  const remove = async () => {
    const r = await useWatchlists.getState().removeItem(item.token);
    // null: nothing was removed, or the call finished after a sign-out (Ruling 104 m3) — no Undo then.
    if (!r) return;
    // P0-4 shows info toasts for TOAST_DURATION_MS.info = 5 s, the FR-WATCH-005 AC3 undo window. Undo puts the
    // row back at the position Go reported (RemovedItem.position).
    pushToast({
      level: 'info',
      message: copy.removed(name, listName),
      action: { label: copy.undo, onSelect: () => void useWatchlists.getState().restoreItem(r).catch(reportError(copy.undoFailed)) },
    });
  };
  const onRemove = () => void remove().catch(reportError(copy.removeFailed));

  const dragHandlers: NonNullable<InstrumentRowProps['dragHandlers']> = {
    draggable: true,
    onDragStart: (e: DragEvent<HTMLTableRowElement>) => { dragged.current = item.token; e.dataTransfer?.setData('text/plain', String(item.token)); },
    onDragOver: (e: DragEvent<HTMLTableRowElement>) => e.preventDefault(),
    onDrop: (e: DragEvent<HTMLTableRowElement>) => {
      e.preventDefault();
      const from = dragged.current;
      dragged.current = null;
      if (from === null || from === item.token) return;
      // The drop target's index in the list the store reorders, read at drop time by token (Ruling 84).
      const list = activeList(useWatchlists.getState());
      const index = list ? list.items.findIndex((i) => i.token === item.token) : -1;
      if (index >= 0) useWatchlists.getState().moveTo(from, index).catch(reportError(copy.moveFailed));
    },
  };

  if (item.missing) {
    return (
      <tr aria-selected={false} aria-label={label ? `${name} · ${copy.missing}` : copy.missing} tabIndex={0}
        data-token={item.token} className="instrument-row instrument-row--missing" onFocus={onFocusRow} {...dragHandlers}>
        <td colSpan={2}>
          {label && <InstrumentLabel instrument={label} />}
          <span className="instrument-row__missing">{copy.missing}</span>
        </td>
        <td className="instrument-row__actions">
          <IconButton label={copy.removeMissing} icon="close" onClick={onRemove} />
        </td>
      </tr>
    );
  }
  if (!label) return null; // unreachable: a non-missing item always has a symbol
  const select = () => { useWatchlists.getState().select(item.token); onChartItem(item); };
  return (
    <InstrumentRow instrument={label} selected={selected} polled={polled} onSelect={select} onChart={select}
      onRemove={onRemove} onFocusRow={onFocusRow} dragHandlers={dragHandlers} />
  );
}
