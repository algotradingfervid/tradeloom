import { useEffect, useId, useRef, useState, type KeyboardEvent, type RefObject } from 'react';
import { Dialog } from '../../components/Dialog';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { searchInstruments } from '../../api/instruments';
import { symbolSearchCopy as copy } from '../../i18n/copy';
import { displayName, EXCHANGE_TAGS, isInScope } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { resolveIdentifier, useInstruments } from '../../store/instrumentsStore';
import './symbolSearch.css';

export const SEARCH_DEBOUNCE_MS = 120;
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
  // Answers are keyed by the query they belong to, so a change of query hides them without a reset.
  const [shown, setShown] = useState<{ q: string; list: Instrument[] } | null>(null);
  const [searchError, setSearchError] = useState<{ q: string; message: string } | null>(null);
  const [chipError, setChipError] = useState<string | null>(null);
  const [active, setActive] = useState(0);
  const seq = useRef(0); // latest search; an older answer is dropped
  const chipSeq = useRef(0); // latest chip lookup
  const alive = useRef(true); // false once the dialog closed: late answers resolve quietly

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
      seq.current += 1;
      chipSeq.current += 1;
    };
  }, []);

  const q = query.trim();
  useEffect(() => {
    const my = ++seq.current;
    if (q === '') return; // Go answers [] for an empty query: never sent
    const t = setTimeout(() => {
      searchInstruments(q, EXCHANGE_TAGS, SEARCH_LIMIT).then(
        (list) => {
          if (my !== seq.current) return;
          setShown({ q, list: list.filter(isInScope) }); // NFR-15: never offer an option, MCX or currency row
          setActive(0);
        },
        (e: unknown) => {
          if (my === seq.current) setSearchError({ q, message: message(e) });
        },
      );
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [q]);

  const results = q === '' || !shown ? [] : shown.list;
  const fresh = q !== '' && shown?.q === q;
  const current = results.length === 0 ? -1 : Math.min(active, results.length - 1);
  const optionId = (i: Instrument) => `${listId}-opt-${i.token}`;
  const activeOption = current >= 0 ? results[current] : undefined;
  const activeId = activeOption ? optionId(activeOption) : undefined;
  const error = chipError ?? (searchError && searchError.q === q ? searchError.message : null);

  useEffect(() => {
    const el = activeId ? document.getElementById(activeId) : null;
    if (el && typeof el.scrollIntoView === 'function') el.scrollIntoView({ block: 'nearest' }); // absent in jsdom
  }, [activeId]);

  const choose = (i: Instrument) => {
    if (!alive.current || !isInScope(i)) return;
    useInstruments.getState().put([i]);
    onSelect(i);
    onClose();
  };

  const chooseChip = (id: string) => {
    const my = ++chipSeq.current;
    setChipError(null);
    resolveIdentifier(id).then(
      (i) => { if (my === chipSeq.current) choose(i); },
      (e: unknown) => { if (my === chipSeq.current) setChipError(message(e)); },
    );
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (results.length > 0) setActive(Math.min(current + 1, results.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (results.length > 0) setActive(Math.max(current - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      // Only rows answering the text in the box: Enter right after typing never picks a row of the previous query.
      if (fresh && activeOption) choose(activeOption);
    }
    // Escape is the Dialog's (it closes and returns focus to the opener).
  };

  let status = '';
  if (fresh) status = results.length === 0 ? copy.noMatch : copy.count(results.length);

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
        aria-expanded={results.length > 0}
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
      <p role="status" aria-live="polite" className="symbol-search__hint">{status}</p>
      <div className="symbol-search__head" aria-hidden="true"><span>{copy.colInstrument}</span><span>{copy.colCompany}</span></div>
      <div id={listId} role="listbox" aria-label={copy.results} className="symbol-search__results">
        {results.map((r, k) => (
          <div
            key={r.token}
            id={optionId(r)}
            role="option"
            aria-selected={k === current}
            className={`symbol-search__row${k === current ? ' is-active' : ''}`}
            onMouseDown={(e) => e.preventDefault()} // keep focus in the input
            onMouseEnter={() => setActive(k)}
            onClick={() => choose(r)}
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
