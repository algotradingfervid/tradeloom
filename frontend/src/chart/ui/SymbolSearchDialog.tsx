import { useCallback, useEffect, useId, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import { Dialog } from '../../components/Dialog';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { instrumentSearchCopy, symbolSearchCopy as copy } from '../../i18n/copy';
import { displayName, isInScope } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useInstrumentSearch } from '../../instruments/useInstrumentSearch';
import { resolveIdentifier, useInstruments } from '../../store/instrumentsStore';
import './symbolSearch.css';

export { SEARCH_DEBOUNCE_MS } from '../../instruments/useInstrumentSearch';
/** Go's maxSearchLimit (internal/service/chart.go): a larger limit is clamped to it. */
const SEARCH_LIMIT = 50;
export const TYPE_TO_SEARCH = 'Type to search';
export const QUICK_CHIPS = [
  { id: 'NSE:NIFTY 50', label: 'NIFTY 50' },
  { id: 'NSE:NIFTY BANK', label: 'NIFTY BANK' },
  { id: 'BSE:SENSEX', label: 'SENSEX' },
  { id: 'BSE:BANKEX', label: 'BANKEX' },
] as const;

const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

export interface SymbolSearchDialogProps {
  open: boolean;
  onClose(): void;
  onSelect(inst: Instrument): void;
}

/**
 * FR-CHART-005: single-select instrument search. The body mounts on every opening (Dialog renders nothing while
 * closed), so each opening starts empty; the Dialog traps focus, closes on Escape/×/backdrop and returns focus to the
 * opener, and its aria-modal blocks the global shortcuts (Ruling 94).
 */
export function SymbolSearchDialog({ open, onClose, onSelect }: SymbolSearchDialogProps) {
  const inputRef = useRef<HTMLInputElement>(null);
  return (
    <Dialog open={open} title={copy.title} onClose={onClose} initialFocus={inputRef} size="lg">
      <SymbolSearch inputRef={inputRef} onClose={onClose} onSelect={onSelect} />
    </Dialog>
  );
}

function SymbolSearch({ inputRef, onClose, onSelect }: { inputRef: RefObject<HTMLInputElement | null>; onClose(): void; onSelect(i: Instrument): void }) {
  const listId = useId();
  const [query, setQuery] = useState('');
  const [chipError, setChipError] = useState<string | null>(null);
  const chipSeq = useRef(0); // latest chip lookup; bumped on unmount and reset
  const alive = useRef(true); // false once the dialog closed: late answers resolve quietly

  // A reset (logout or user switch) silences the chip lookup in flight and empties the box (Ruling 112 I2).
  const onReset = useCallback(() => {
    chipSeq.current += 1;
    setChipError(null);
    setQuery('');
  }, []);
  const search = useInstrumentSearch(query, { limit: SEARCH_LIMIT, onReset });
  const { q, rows, fresh } = search;

  // The highlight belongs to the rows it was set on: new rows start at the top.
  const [highlight, setHighlight] = useState<{ rows: readonly Instrument[]; index: number }>({ rows, index: 0 });
  const current = rows.length === 0 ? -1 : highlight.rows === rows ? Math.min(highlight.index, rows.length - 1) : 0;
  const setActive = (index: number) => setHighlight({ rows, index });

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      chipSeq.current += 1;
    };
  }, []);

  const optionId = (i: Instrument) => `${listId}-opt-${i.token}`;
  const activeOption = current >= 0 ? rows[current] : undefined;
  const activeId = activeOption ? optionId(activeOption) : undefined;
  const error = chipError ?? search.error;
  const pending = q !== '' && !fresh && search.error === null;

  useEffect(() => {
    const el = activeId ? document.getElementById(activeId) : null;
    if (el && typeof el.scrollIntoView === 'function') el.scrollIntoView({ block: 'nearest' }); // absent in jsdom
  }, [activeId]);

  const select = (i: Instrument) => {
    if (!alive.current || !isInScope(i)) return;
    onSelect(i);
    onClose();
  };

  /** A search row: cached here, but only while the answer is still current (Ruling 107). */
  const chooseRow = (i: Instrument) => {
    if (!search.isCurrent()) return;
    useInstruments.getState().put([i]);
    select(i);
  };

  /** A chip: resolveIdentifier already caches it while current, so it is not put again. */
  const chooseChip = (id: string) => {
    const my = ++chipSeq.current;
    setChipError(null);
    resolveIdentifier(id).then(
      (i) => { if (my === chipSeq.current) select(i); },
      (e: unknown) => { if (my === chipSeq.current) setChipError(message(e)); },
    );
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (rows.length > 0) setActive(Math.min(current + 1, rows.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (rows.length > 0) setActive(Math.max(current - 1, 0));
    } else if (e.key === 'Enter') {
      if (e.nativeEvent.isComposing || e.keyCode === 229) return; // IME composition: Enter commits the text
      e.preventDefault();
      // Only rows answering the text in the box: Enter while the next query is pending never picks an older row.
      if (fresh && activeOption) chooseRow(activeOption);
    }
    // Escape is the Dialog's (it closes and returns focus to the opener).
  };

  return (
    <div className="symbol-search">
      <input
        ref={inputRef}
        type="text"
        className="field__input symbol-search__input"
        placeholder={copy.placeholder}
        aria-label={copy.inputLabel}
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={rows.length > 0}
        aria-controls={listId}
        aria-activedescendant={activeId}
        autoComplete="off"
        spellCheck={false}
        value={query}
        onChange={(e) => { setQuery(e.target.value); setChipError(null); }}
        onKeyDown={onKeyDown}
      />
      <div className="symbol-search__chips" role="group" aria-label={copy.quickChips}>
        {QUICK_CHIPS.map((c) => (
          <button key={c.id} type="button" className="symbol-search__chip" onClick={() => chooseChip(c.id)}>{c.label}</button>
        ))}
      </div>
      {error && <p role="alert" className="symbol-search__error">{error}</p>}
      {q === '' && <p className="symbol-search__hint">{TYPE_TO_SEARCH}</p>}
      {fresh && rows.length === 0 && <p className="symbol-search__hint" aria-hidden="true">{instrumentSearchCopy.noMatch}</p>}
      {/* Always rendered, so the polite announcement is never lost to display:none (Ruling 112 I1). */}
      <p role="status" aria-live="polite" className="visually-hidden">{search.status}</p>
      <div className="symbol-search__head" aria-hidden="true"><span>{copy.colInstrument}</span><span>{copy.colCompany}</span></div>
      <div id={listId} role="listbox" aria-label={copy.results} aria-busy={pending || undefined} className="symbol-search__results">
        {rows.map((r, k) => (
          <div
            key={r.token}
            id={optionId(r)}
            role="option"
            aria-selected={k === current}
            className={`symbol-search__row${k === current ? ' is-active' : ''}`}
            onMouseDown={(e) => e.preventDefault()} // keep focus in the input
            onMouseMove={() => { if (k !== current) setActive(k); }}
            onClick={() => chooseRow(r)}
          >
            <InstrumentLabel instrument={r} />
            {/* Indices carry their symbol as name: the column stays empty rather than repeating it. */}
            <span className="symbol-search__company">{r.name !== displayName(r) ? r.name : ''}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
