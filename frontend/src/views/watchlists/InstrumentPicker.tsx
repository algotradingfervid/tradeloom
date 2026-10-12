import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import { Button } from '../../components/Button';
import { Dialog } from '../../components/Dialog';
import { IconButton } from '../../components/IconButton';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { PriceChange } from '../../components/instrument/PriceChange';
import { instrumentPickerCopy } from '../../i18n/copy';
import { EXCHANGE_TAGS, displayName, isInScope, type ExchangeTag } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { instrumentSearchCopy, useInstrumentSearch } from '../../instruments/useInstrumentSearch';
import { useSubscription } from '../../market/hooks';
import { ensureInstruments, useInstruments } from '../../store/instrumentsStore';
import { registerReset } from '../../store/reset';
import { pushToast } from '../../store/toasts';
import { tokensOf, useWatchlists } from '../../store/watchlistsStore';
import { nextWatchlistName } from './names';
import './instrumentPicker.css';

/** Go's maxSearchLimit (internal/service/chart.go): a larger limit is clamped to it. */
const SEARCH_LIMIT = 50;

const copy = instrumentPickerCopy;

// Resets (logout or user switch) seen by this module: a save that awaited across one is never reported (Ruling 118 I2).
let resets = 0;
registerReset(() => { resets += 1; }, { name: 'instrumentPickerResets', inspect: () => ({ resets }) });

export interface InstrumentPickerProps {
  open: boolean;
  title?: string;
  /** Caller maximum (FR-WATCH-007 AC2); none = unlimited (AC3). */
  max?: number;
  /** Tags offered (default: all five in-scope tags). */
  segments?: readonly ExchangeTag[];
  /** The selection the picker opens with (read on each opening; deduped by token and clamped to `max`). */
  initial?: readonly Instrument[];
  onClose(): void;
  onDone(selected: Instrument[]): void;
}

/**
 * FR-WATCH-007: the shared multi-select instrument picker (watchlists now; the P1 equity builder and P3 scanner custom
 * symbols reuse it). The body mounts on every opening (Dialog renders nothing while closed), so each opening starts from
 * `initial`, and closing unmounts it, which releases its `widget` subscription. The Dialog traps focus, owns Escape,
 * × and the backdrop, and returns focus to the opener.
 */
export function InstrumentPicker({ open, ...rest }: InstrumentPickerProps) {
  return open ? <PickerDialog {...rest} /> : null;
}

type Source = { kind: 'search' } | { kind: 'watchlist'; id: string };
type ChipRows = { id: string; key: string; rows: readonly Instrument[] };

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));
const NONE: readonly Instrument[] = [];

/**
 * The opening selection: in scope only (NFR-15), one per token, at most `max`. The caller owns the maximum; an
 * `initial` longer than it is cut here rather than shown over the limit.
 */
function openingSelection(initial: readonly Instrument[], max: number | undefined): Instrument[] {
  const seen = new Set<number>();
  const out: Instrument[] = [];
  for (const i of initial) {
    if (!isInScope(i) || seen.has(i.token)) continue;
    if (max !== undefined && out.length >= max) break;
    seen.add(i.token);
    out.push(i);
  }
  return out;
}

function PickerDialog({ title = copy.title, max, segments = EXCHANGE_TAGS, initial = NONE, onClose, onDone }: Omit<InstrumentPickerProps, 'open'>) {
  const inputRef = useRef<HTMLInputElement>(null);
  const saveErrorId = useId();
  const gridRef = useRef<HTMLDivElement>(null);
  const selectedRef = useRef<HTMLUListElement>(null);
  const saveAsRef = useRef<HTMLButtonElement>(null);
  const lists = useWatchlists((s) => s.lists);
  const [query, setQuery] = useState('');
  const [source, setSource] = useState<Source>({ kind: 'search' });
  const [chip, setChip] = useState<ChipRows | null>(null);
  const [chipError, setChipError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Instrument[]>(() => openingSelection(initial, max));
  // `n` keys the alert, so a repeated refusal mounts a new one and is announced again.
  const [limitMsg, setLimitMsg] = useState<{ text: string; n: number } | null>(null);
  const [chipNonce, setChipNonce] = useState(0); // clicking a chip again retries its load
  const [active, setActive] = useState(0);
  const [naming, setNaming] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const savingRef = useRef(false); // set synchronously: a second Enter in the same frame does not save twice
  const chipSeq = useRef(0); // latest chip load; bumped on reset, on a new source and on unmount
  const alive = useRef(true);
  const focusAfter = useRef<(() => void) | null>(null);
  const tags = segments.length > 0 ? segments : EXCHANGE_TAGS; // [] means all, as in Go
  const tagsKey = tags.join(',');

  // A reset (logout or user switch) clears the picker in place (Ruling 118 I1): the answers and the chip load in flight
  // are dropped, and the previous user's selection goes too, so its tokens leave the widget set and Done cannot write
  // them into the next user's list. The picker stays open.
  const onReset = useCallback(() => {
    chipSeq.current += 1;
    setQuery('');
    setSource({ kind: 'search' });
    setChip(null);
    setChipError(null);
    setSelected([]);
    setLimitMsg(null);
    setNaming(null);
    setSaveError(null);
    setActive(0);
  }, []);
  // While a watchlist chip is the source no search is sent ('' is never searched).
  const search = useInstrumentSearch(source.kind === 'search' ? query : '', { limit: SEARCH_LIMIT, tags, onReset });

  const chipList = source.kind === 'watchlist' ? lists.find((l) => l.id === source.id) : undefined;
  const chipKey = chipList ? tokensOf(chipList).join(',') : '';
  const chipId = source.kind === 'watchlist' ? source.id : null;

  // Watchlist items carry display data only: load the full instruments (ensureInstruments caches them, in scope only,
  // with its negative cache for tokens Go omits), then offer those inside the caller's tags.
  useEffect(() => {
    if (chipId === null) return;
    const my = ++chipSeq.current;
    const tokens = chipKey === '' ? [] : chipKey.split(',').map(Number);
    const allowed = tagsKey.split(',');
    ensureInstruments(tokens).then(
      () => {
        if (my !== chipSeq.current) return;
        const { byToken } = useInstruments.getState();
        const rows = tokens.map((t) => byToken[t]).filter((i): i is Instrument => i !== undefined && isInScope(i) && allowed.includes(i.tag));
        setChip({ id: chipId, key: chipKey, rows });
        setChipError(null);
      },
      (e: unknown) => { if (my === chipSeq.current) setChipError(message(e)); },
    );
    return () => { chipSeq.current += 1; };
    // chipNonce: the same chip clicked again retries (e.g. after a failed load).
  }, [chipId, chipKey, tagsKey, chipNonce]);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      chipSeq.current += 1;
    };
  }, []);

  // The selection's live prices (Selected pane); released when the picker closes (the body unmounts).
  useSubscription('instrument-picker', 'widget', selected.map((s) => s.token));

  // Focus moves after a render that removed the focused control (a removed row, the closed name field).
  useEffect(() => {
    const f = focusAfter.current;
    focusAfter.current = null;
    f?.();
  });

  const chipRows = chip !== null && chip.id === chipId && chip.key === chipKey ? chip.rows : null;
  const rows: readonly Instrument[] = source.kind === 'search' ? search.rows : (chipRows ?? NONE);
  const fromSearch = source.kind === 'search';
  const pending = fromSearch ? search.q !== '' && !search.fresh && search.error === null : chipRows === null && chipError === null;
  const error = fromSearch ? search.error : chipError;
  const current = rows.length === 0 ? -1 : Math.min(active, rows.length - 1);
  const status = fromSearch ? search.status : chipRows === null ? '' : chipRows.length === 0 ? copy.emptyList : instrumentSearchCopy.count(chipRows.length);

  const selectedTokens = new Set(selected.map((s) => s.token));
  const isSelected = (token: number) => selectedTokens.has(token);
  const allListed = rows.length > 0 && rows.every((r) => isSelected(r.token));

  /** Adds rows in order, ignoring duplicates, up to the caller maximum. Search rows are cached only while current. */
  const addMany = (items: readonly Instrument[]) => {
    if (fromSearch && !search.isCurrent()) return;
    const next = [...selected];
    const added: Instrument[] = [];
    let refused = false;
    for (const i of items) {
      if (!isInScope(i) || next.some((s) => s.token === i.token)) continue;
      if (max !== undefined && next.length >= max) { refused = true; break; }
      next.push(i);
      added.push(i);
    }
    if (fromSearch && added.length > 0) useInstruments.getState().put(added);
    setSelected(next);
    setLimitMsg((m) => (refused && max !== undefined ? { text: copy.limit(max), n: (m?.n ?? 0) + 1 } : null));
  };
  const removeTokens = (tokens: ReadonlySet<number>) => {
    setSelected((s) => s.filter((i) => !tokens.has(i.token)));
    setLimitMsg(null);
  };
  const toggle = (r: Instrument) => (isSelected(r.token) ? removeTokens(new Set([r.token])) : addMany([r]));
  const toggleAll = () => (allListed ? removeTokens(new Set(rows.map((r) => r.token))) : addMany(rows));

  /** The Selected pane's remove button: focus goes to the next one (else the previous, else the search box). */
  const removeSelected = (index: number) => {
    const neighbour = selected[index + 1] ?? selected[index - 1];
    focusAfter.current = () => {
      const btn = neighbour ? selectedRef.current?.querySelector<HTMLElement>(`[data-remove-token="${neighbour.token}"]`) : null;
      (btn ?? inputRef.current)?.focus();
    };
    removeTokens(new Set([selected[index].token]));
  };

  const chooseList = (id: string) => {
    setQuery('');
    setActive(0);
    setSource({ kind: 'watchlist', id });
    setChipNonce((n) => n + 1);
  };

  // Grid pattern (Ruling 115 I3): one tab stop on the active row's checkbox; arrows, Home and End move it.
  const focusRow = (index: number) => {
    setActive(index);
    gridRef.current?.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')[index]?.focus();
  };
  const onInputKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' && rows.length > 0) {
      e.preventDefault();
      focusRow(Math.max(current, 0));
    }
    // Escape is the Dialog's: it closes the picker once and returns focus to the opener.
  };
  const onGridKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    const last = rows.length - 1;
    if (last < 0) return;
    let to: number | null = null;
    if (e.key === 'ArrowDown') to = Math.min(current + 1, last);
    else if (e.key === 'ArrowUp') {
      if (current <= 0) {
        e.preventDefault();
        inputRef.current?.focus();
        return;
      }
      to = current - 1;
    } else if (e.key === 'Home') to = 0;
    else if (e.key === 'End') to = last;
    else if ((e.key === 'Enter' || e.key === ' ') && current >= 0) {
      // Handled here (not by the native checkbox) so Space and Enter act alike and Space does not toggle twice.
      e.preventDefault();
      toggle(rows[current]);
      return;
    }
    if (to === null) return;
    e.preventDefault();
    focusRow(to);
  };

  const startNaming = () => {
    setNaming(nextWatchlistName(lists.map((l) => l.name)));
    setSaveError(null);
  };
  const stopNaming = () => {
    setNaming(null);
    setSaveError(null);
    focusAfter.current = () => saveAsRef.current?.focus();
  };
  const saveAsWatchlist = async () => {
    if (naming === null || savingRef.current) return;
    const name = naming;
    savingRef.current = true;
    setSaving(true);
    const at = resets;
    // activate = false: Go's Create activates the new list, the store puts the previous one back (Ruling 87).
    const err = await useWatchlists.getState().create(name, selected.map((s) => s.token), false);
    savingRef.current = false;
    // The store answers null also for a call dropped by a reset: such a save is neither reported nor shown as failed.
    if (resets !== at) {
      if (alive.current) setSaving(false);
      return;
    }
    if (err === null) pushToast({ level: 'info', message: copy.created(name.trim()) });
    if (!alive.current) return;
    setSaving(false);
    if (err !== null) { setSaveError(err); return; }
    stopNaming();
  };

  const footer = (
    <>
      {naming === null ? (
        <Button ref={saveAsRef} disabled={selected.length === 0} onClick={startNaming}>{copy.saveAs}</Button>
      ) : (
        <span className="picker__save">
          <input
            className="field__input"
            aria-label={copy.name}
            aria-invalid={saveError !== null || undefined}
            aria-describedby={saveError !== null ? saveErrorId : undefined}
            autoFocus
            value={naming}
            onChange={(e) => { setNaming(e.target.value); setSaveError(null); }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.nativeEvent.isComposing) { e.preventDefault(); void saveAsWatchlist(); }
              // Handled here, once: Escape in the name field cancels naming; it does not also close the picker.
              if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); stopNaming(); }
            }}
          />
          <Button aria-disabled={saving || undefined} onClick={() => void saveAsWatchlist()}>{copy.save}</Button>
          <Button variant="ghost" onClick={stopNaming}>{copy.cancel}</Button>
          {saveError !== null && <span id={saveErrorId} role="alert" className="field__error">{saveError}</span>}
        </span>
      )}
      <Button variant="primary" onClick={() => onDone(selected)}>{copy.done}</Button>
    </>
  );

  return (
    <Dialog open title={title} onClose={onClose} footer={footer} size="lg" initialFocus={inputRef as RefObject<HTMLElement | null>}>
      <div className="picker">
        <div className="picker__left">
          <input
            ref={inputRef}
            type="text"
            className="field__input picker__input"
            aria-label={copy.search}
            placeholder={copy.placeholder}
            autoComplete="off"
            spellCheck={false}
            value={query}
            onChange={(e) => { setQuery(e.target.value); setSource({ kind: 'search' }); setActive(0); }}
            onKeyDown={onInputKeyDown}
          />
          {lists.length > 0 && (
            <div className="picker__chips" role="group" aria-label={copy.lists}>
              {lists.map((l) => (
                <button key={l.id} type="button" className="picker__chip" aria-pressed={chipId === l.id} onClick={() => chooseList(l.id)}>
                  {l.name}
                </button>
              ))}
            </div>
          )}
          {error !== null && <p role="alert" className="picker__error">{error}</p>}
          {fromSearch && search.q === '' && <p className="picker__hint">{copy.typeToSearch}</p>}
          {/* Always rendered, so the polite announcement is never lost to display:none (Ruling 112 I1). */}
          <p role="status" aria-live="polite" className="visually-hidden">{status}</p>
          <label className="picker__all">
            <input type="checkbox" aria-label={copy.selectAll} checked={allListed} disabled={rows.length === 0} onChange={toggleAll} />
            <span aria-hidden="true">{copy.selectAll}</span>
          </label>
          <div
            ref={gridRef}
            role="grid"
            aria-label={copy.results}
            aria-multiselectable="true"
            aria-rowcount={rows.length}
            aria-busy={pending || undefined}
            className="picker__results"
            onKeyDown={onGridKeyDown}
          >
            {rows.map((r, k) => (
              <div key={r.token} role="row" aria-selected={isSelected(r.token)} aria-rowindex={k + 1} className={`picker__row${k === current ? ' is-active' : ''}`}>
                <span role="gridcell" className="picker__cell">
                  <input
                    type="checkbox"
                    aria-label={copy.select(displayName(r))}
                    tabIndex={k === current ? 0 : -1}
                    checked={isSelected(r.token)}
                    onFocus={() => { if (k !== current) setActive(k); }}
                    onChange={() => toggle(r)}
                  />
                </span>
                <span role="gridcell" className="picker__cell picker__cell--label">
                  <InstrumentLabel instrument={r} showName />
                </span>
              </div>
            ))}
          </div>
        </div>
        <div className="picker__right">
          <h3 className="picker__heading" aria-live="polite">{copy.selected(selected.length, max)}</h3>
          {limitMsg !== null && <p key={limitMsg.n} role="alert" className="picker__error">{limitMsg.text}</p>}
          {selected.length === 0 && <p className="picker__hint">{copy.noneSelected}</p>}
          <ul ref={selectedRef} aria-label={copy.selectedList} className="picker__selected">
            {selected.map((s, k) => (
              <li key={s.token} className="picker__selected-row">
                <InstrumentLabel instrument={s} />
                <PriceChange token={s.token} />
                <IconButton label={copy.remove(displayName(s))} icon="close" size={14} data-remove-token={s.token} onClick={() => removeSelected(k)} />
              </li>
            ))}
          </ul>
        </div>
      </div>
    </Dialog>
  );
}
