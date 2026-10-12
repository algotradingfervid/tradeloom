import { useEffect, useRef, useState, type DragEvent, type FocusEvent, type KeyboardEvent, type MutableRefObject } from 'react';
import type { WatchlistItem } from '../../api/backend';
import { parseBackendError } from '../../api/errors';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { InstrumentRow, type InstrumentRowProps } from '../../components/instrument/InstrumentRow';
import { IconButton } from '../../components/IconButton';
import { watchlistRowsCopy as copy } from '../../i18n/copy';
import { displayName, labelOfItem } from '../../instruments/naming';
import { usePolled, useSubscription } from '../../market/hooks';
import { useShortcut } from '../../shortcuts/ShortcutProvider';
import { formatBinding } from '../../shortcuts/registry';
import { useShortcutStore } from '../../shortcuts/store';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists, type Watchlist } from '../../store/watchlistsStore';
import './watchlists.css';

const reason = (e: unknown): string => (e instanceof Error ? e.message : parseBackendError(e).reason);
const reportError = (text: (r: string) => string) => (e: unknown) => {
  pushToast({ level: 'error', message: text(reason(e)) });
};
/** A missing item in Go's shape has no symbol; one that still carries a symbol shows it (labelOfItem never throws). */
const nameOf = (item: WatchlistItem): string => (item.symbol ? displayName(labelOfItem(item)) : copy.missingName);
const REORDER_IDS = ['watchlist.moveUp', 'watchlist.moveDown'] as const;

/** Registered only while a row has focus, so Alt+Up/Down stay free everywhere else (P0-4 Ruling 94 M1). */
function ReorderShortcuts({ token, onMove }: { token: number; onMove(token: number, delta: -1 | 1): void }) {
  useShortcut('watchlist.moveUp', () => onMove(token, -1));
  useShortcut('watchlist.moveDown', () => onMove(token, 1));
  return null;
}

/**
 * The rows of one watchlist. The store's remove/reorder act on the active list, so `list` must be it; any other list
 * renders read-only (no remove, drag or reorder shortcuts) and logs in dev (Ruling 115 m7).
 */
export function WatchlistRows({ list, onChartItem }: { list: Watchlist; onChartItem(item: WatchlistItem): void }) {
  // Missing items (token no longer in the master) cannot stream; everything else is the watchlist consumer's set.
  useSubscription('watchlist-panel', 'watchlist', list.items.filter((i) => !i.missing).map((i) => i.token));
  const selected = useWatchlists((s) => s.selectedToken);
  const isActive = useWatchlists((s) => activeList(s)?.id === list.id);
  const keyShortcuts = useShortcutStore((s) =>
    REORDER_IDS.map((id) => formatBinding(s.actions.find((a) => a.id === id)?.binding ?? '')).filter(Boolean).join(' '));
  // Rows are tracked by token, never by index (P0-4 Ruling 84); `index` is only the fallback when the row is gone.
  const [roving, setRoving] = useState<{ token: number | null; index: number }>({ token: null, index: 0 });
  const [focused, setFocused] = useState<number | null>(null);
  const [announcement, setAnnouncement] = useState('');
  const dragged = useRef<number | null>(null);
  const body = useRef<HTMLTableSectionElement>(null);

  useEffect(() => {
    if (!isActive && import.meta.env.DEV) console.error(copy.readOnlyMisuse, list.id);
  }, [isActive, list.id]);

  const tokens = list.items.map((i) => i.token);
  const found = roving.token === null ? -1 : tokens.indexOf(roving.token);
  const rovingToken = tokens[found >= 0 ? found : Math.min(roving.index, Math.max(0, tokens.length - 1))];

  const announceMove = (token: number) => {
    const now = activeList(useWatchlists.getState());
    const at = now ? now.items.findIndex((i) => i.token === token) : -1;
    if (now && at >= 0) setAnnouncement(copy.moved(nameOf(now.items[at]), at + 1, now.items.length));
  };
  const move = (token: number, delta: -1 | 1) => {
    // React keeps focus on the moved row across the re-render (it restores the focused node after the commit).
    useWatchlists.getState().move(token, delta).then(() => announceMove(token), reportError(copy.moveFailed));
  };
  const moveTo = (token: number, index: number) => {
    useWatchlists.getState().moveTo(token, index).then(() => announceMove(token), reportError(copy.moveFailed));
  };

  const focusRow = (index: number) => {
    const token = tokens[Math.max(0, Math.min(tokens.length - 1, index))];
    body.current?.querySelector<HTMLElement>(`tr[data-token="${token}"]`)?.focus();
  };
  const onRowKeyDown = (e: KeyboardEvent<HTMLTableRowElement>, token: number) => {
    // Modified arrows belong to the shortcuts (Alt+ArrowUp/Down reorder), not to row navigation.
    if (e.target !== e.currentTarget || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
    const i = tokens.indexOf(token);
    const moves: Record<string, number> = { ArrowDown: i + 1, ArrowUp: i - 1, Home: 0, End: tokens.length - 1 };
    const target = moves[e.key];
    if (target === undefined) return;
    e.preventDefault();
    focusRow(target);
  };

  const onFocus = (e: FocusEvent<HTMLTableSectionElement>) => {
    const row = (e.target as Element).closest<HTMLElement>('tr[data-token]');
    if (!row) return;
    const token = Number(row.dataset.token);
    setFocused(token);
    setRoving({ token, index: tokens.indexOf(token) });
  };
  const onBlur = (e: FocusEvent<HTMLTableSectionElement>) => {
    if (!(e.relatedTarget instanceof Node && e.currentTarget.contains(e.relatedTarget))) setFocused(null);
  };

  if (list.items.length === 0) return <p className="watchlist-empty">{copy.empty}</p>;

  return (
    <>
      <table className="watchlist-table" role="grid" aria-label={copy.tableLabel(list.name)}>
        <thead>
          <tr><th scope="col">{copy.symbolColumn}</th><th scope="col">{copy.priceColumn}</th><th scope="col"><span className="visually-hidden">{copy.actionsColumn}</span></th></tr>
        </thead>
        <tbody ref={body} onFocus={onFocus} onBlur={onBlur}>
          {list.items.map((item, index) => (
            <WatchlistRow key={item.token} item={item} position={index + 1} listName={list.name} selected={selected === item.token}
              tabbable={item.token === rovingToken} readOnly={!isActive} keyShortcuts={isActive ? keyShortcuts : ''}
              onChartItem={onChartItem} onRowKeyDown={(e) => onRowKeyDown(e, item.token)} onDrop={moveTo} dragged={dragged} />
          ))}
        </tbody>
      </table>
      {isActive && focused !== null && tokens.includes(focused) && <ReorderShortcuts token={focused} onMove={move} />}
      <p role="status" className="visually-hidden">{announcement}</p>
    </>
  );
}

interface RowProps {
  item: WatchlistItem; position: number; listName: string; selected: boolean; tabbable: boolean; readOnly: boolean; keyShortcuts: string;
  onChartItem(item: WatchlistItem): void; onRowKeyDown(e: KeyboardEvent<HTMLTableRowElement>): void;
  onDrop(token: number, index: number): void; dragged: MutableRefObject<number | null>;
}

function WatchlistRow({ item, position, listName, selected, tabbable, readOnly, keyShortcuts, onChartItem, onRowKeyDown, onDrop, dragged }: RowProps) {
  const polled = usePolled(item.token);
  const name = nameOf(item);

  const remove = async () => {
    const r = await useWatchlists.getState().removeItem(item.token);
    // null: nothing was removed, or the call finished after a sign-out (Ruling 104 m3) — no toast then.
    if (!r) return;
    // A missing item cannot be put back: Go's Add refuses a token outside the master (unknown_instrument).
    if (item.missing) {
      pushToast({ level: 'info', message: copy.removed(name, listName) });
      return;
    }
    // A toast with an action stays until it is used or dismissed (P0-4 Ruling 86 M4), longer than FR-WATCH-005's 5 s
    // (accepted, Ruling 115 m1). Undo puts the row back at the position Go reported (RemovedItem.position).
    pushToast({
      level: 'info',
      message: copy.removed(name, listName),
      action: { label: copy.undo, onSelect: () => void useWatchlists.getState().restoreItem(r).catch(reportError(copy.undoFailed)) },
    });
  };
  const onRemove = readOnly ? undefined : () => void remove().catch(reportError(copy.removeFailed));

  const dragHandlers: InstrumentRowProps['dragHandlers'] = readOnly ? undefined : {
    draggable: true,
    onDragStart: (e: DragEvent<HTMLTableRowElement>) => { dragged.current = item.token; e.dataTransfer?.setData('text/plain', String(item.token)); },
    onDragOver: (e: DragEvent<HTMLTableRowElement>) => e.preventDefault(),
    onDragEnd: () => { dragged.current = null; },
    onDrop: (e: DragEvent<HTMLTableRowElement>) => {
      e.preventDefault();
      const from = dragged.current;
      dragged.current = null;
      if (from === null || from === item.token) return;
      // The drop target's index in the list the store reorders, read at drop time by token (Ruling 84).
      const list = activeList(useWatchlists.getState());
      const index = list ? list.items.findIndex((i) => i.token === item.token) : -1;
      if (index >= 0) onDrop(from, index);
    },
  };

  if (item.missing) {
    return (
      <tr aria-selected={false} aria-label={item.symbol ? `${name} · ${copy.missing}` : copy.missing} tabIndex={tabbable ? 0 : -1}
        aria-keyshortcuts={keyShortcuts || undefined} data-token={item.token} className="instrument-row instrument-row--missing"
        onKeyDown={onRowKeyDown} {...dragHandlers}>
        <td colSpan={2}>
          {item.symbol && <InstrumentLabel instrument={labelOfItem(item)} />}
          <span className="instrument-row__missing">{copy.missing}</span>
        </td>
        <td className="instrument-row__actions">
          {onRemove && <IconButton label={copy.removeMissing(position)} icon="close" tabIndex={tabbable ? undefined : -1} onClick={onRemove} />}
        </td>
      </tr>
    );
  }
  const select = () => { useWatchlists.getState().select(item.token); onChartItem(item); };
  return (
    <InstrumentRow instrument={labelOfItem(item)} selected={selected} polled={polled} onSelect={select} onChart={select}
      onRemove={onRemove} dragHandlers={dragHandlers} tabbable={tabbable} onRowKeyDown={onRowKeyDown} keyShortcuts={keyShortcuts} />
  );
}
