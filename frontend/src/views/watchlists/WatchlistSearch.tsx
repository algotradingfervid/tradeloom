import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { watchlistSearchCopy as copy } from '../../i18n/copy';
import { displayName } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { instrumentSearchCopy, useInstrumentSearch } from '../../instruments/useInstrumentSearch';
import { useInstruments } from '../../store/instrumentsStore';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import './watchlists.css';

/** Go's default page (internal/service/chart.go defaultSearch); Go caps any limit at 50. */
const SEARCH_LIMIT = 20;

/**
 * FR-WATCH-004: inline search that adds to the active watchlist. A combobox with a listbox popup (WAI-ARIA 1.2
 * list-autocomplete pattern): focus stays in the box, arrows move the highlight, Enter or a click adds, Escape closes
 * the popup and a second Escape clears the box. Searching, the late-answer guard and the reset guard are the shared
 * useInstrumentSearch (Ruling 112). Rows are listed, counted, marked and added only while they answer the current query
 * (`fresh`, Ruling 114 I1). Ranking is Go's (FR-WATCH-008); the order is shown as received.
 */
export function WatchlistSearch() {
  const list = useWatchlists(activeList);
  const id = useId();
  const listboxId = `${id}-results`;
  const optionId = (token: number) => `${id}-opt-${token}`;
  const [query, setQuery] = useState('');
  /** The last add, announced in the polite region until the query changes (Ruling 114 m2). */
  const [notice, setNotice] = useState('');
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  /** Tokens whose Add is in flight: a repeated Enter or click does not send a second Add. */
  const adding = useRef(new Set<number>());

  const clear = () => {
    setQuery('');
    setNotice('');
    setOpen(false);
    setActive(0);
  };
  // A reset (sign-out or user switch) drops the answers in the hook and clears the box, so nothing is searched for a
  // session that has ended; an Add finishing afterwards comes back 'cancelled' and shows nothing (Ruling 104 m1).
  const search = useInstrumentSearch(query, { limit: SEARCH_LIMIT, onReset: clear });

  // A new answer starts at its first row and opens the popup (adjusting state during render).
  const [seenRows, setSeenRows] = useState(search.rows);
  if (seenRows !== search.rows) {
    setSeenRows(search.rows);
    setActive(0);
    if (search.fresh && search.rows.length > 0) setOpen(true);
  }

  const results = search.fresh ? search.rows : [];
  const showList = open && results.length > 0;
  const highlighted = showList ? results[Math.min(active, results.length - 1)] : undefined;
  const highlightedId = highlighted ? optionId(highlighted.token) : undefined;

  useEffect(() => {
    // jsdom has no scrollIntoView; browsers do.
    if (highlightedId) document.getElementById(highlightedId)?.scrollIntoView?.({ block: 'nearest' });
  }, [highlightedId]);

  const onChange = (value: string) => {
    if (value.trim() !== query.trim()) setNotice('');
    setQuery(value);
  };

  const add = async (inst: Instrument) => {
    const target = activeList(useWatchlists.getState());
    if (!target || adding.current.has(inst.token)) return;
    adding.current.add(inst.token);
    try {
      // Rows look the instrument up by token later; an answer from before a reset is never cached.
      if (search.isCurrent()) useInstruments.getState().put([inst]);
      const outcome = await useWatchlists.getState().addItem(inst.token);
      // 'cancelled' (finished after a sign-out) shows nothing (Ruling 104 m1).
      if (outcome === 'duplicate') pushToast({ level: 'info', message: copy.alreadyIn(target.name) });
      else if (outcome === 'added') setNotice(copy.added(displayName(inst), target.name));
    } catch (e) {
      pushToast({ level: 'error', message: copy.addFailed(displayName(inst), e instanceof Error ? e.message : String(e)) });
    } finally {
      adding.current.delete(inst.token);
    }
  };

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    // IME composition keys belong to the input method (as in the shortcut provider, P0-4 Ruling 94 I2).
    if (e.nativeEvent.isComposing || e.keyCode === 229) return;
    switch (e.key) {
      case 'ArrowDown':
        if (results.length === 0) return;
        e.preventDefault();
        if (showList) setActive((a) => Math.min(a + 1, results.length - 1));
        else setOpen(true);
        return;
      case 'ArrowUp':
        if (!showList) return;
        e.preventDefault();
        setActive((a) => Math.max(a - 1, 0));
        return;
      case 'Enter':
        if (!highlighted || e.ctrlKey || e.altKey || e.shiftKey || e.metaKey) return;
        e.preventDefault();
        void add(highlighted);
        return;
      case 'Escape':
        // Each Escape this box handles stays here; one with nothing left to do bubbles (Ruling 114 m1).
        if (showList) {
          e.preventDefault();
          e.stopPropagation();
          setOpen(false);
        } else if (query !== '') {
          e.preventDefault();
          e.stopPropagation();
          clear();
        }
        return;
    }
  };

  // One always-present polite region: the add notice, else a failure, else the hook's count / no-match text.
  const announcement = notice || (search.error !== null ? copy.searchFailed : search.status);
  // "No matches" and a failure are also shown, hidden from assistive tech (the region already announces them).
  const visible =
    search.error !== null ? copy.searchFailed
    : search.fresh && search.rows.length === 0 ? instrumentSearchCopy.noMatch
    : '';

  return (
    <div className="watchlist-search">
      <input
        className="field__input watchlist-search__input"
        type="text"
        role="combobox"
        aria-label={copy.label}
        aria-autocomplete="list"
        aria-expanded={showList}
        aria-controls={showList ? listboxId : undefined}
        aria-activedescendant={highlightedId}
        autoComplete="off"
        spellCheck={false}
        placeholder={copy.placeholder}
        value={query}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={onKeyDown}
        onFocus={() => { if (results.length > 0) setOpen(true); }}
        onBlur={() => setOpen(false)}
      />
      <div role="status" aria-live="polite" className="visually-hidden">{announcement}</div>
      {visible && <div className="watchlist-search__message muted" aria-hidden="true">{visible}</div>}
      {showList && (
        <ul
          id={listboxId}
          role="listbox"
          aria-label={copy.results}
          className="watchlist-search__list"
          onMouseDown={(e) => e.preventDefault()} // keep focus in the box, also on the scrollbar (Ruling 114 m4)
        >
          {results.map((r) => {
            const inList = !!list?.items.some((i) => i.token === r.token);
            const isActive = r === highlighted;
            return (
              <li
                key={r.token}
                id={optionId(r.token)}
                role="option"
                aria-selected={isActive}
                className={`watchlist-search__row${isActive ? ' is-active' : ''}`}
                onMouseEnter={() => setActive(results.indexOf(r))}
                onClick={() => void add(r)}
              >
                <InstrumentLabel instrument={r} showName />
                {list && (
                  <span
                    className={`watchlist-search__add${inList ? ' is-in-list' : ''}`}
                    role="img"
                    aria-label={inList ? copy.alreadyIn(list.name) : copy.addTo(list.name)}
                  >
                    {inList ? '✓' : '+'}
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
