import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { searchInstruments } from '../../api/instruments';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { watchlistSearchCopy as copy } from '../../i18n/copy';
import { displayName, EXCHANGE_TAGS, isInScope } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useInstruments } from '../../store/instrumentsStore';
import { registerReset } from '../../store/reset';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import './watchlists.css';

export const SEARCH_DEBOUNCE_MS = 120;
/** Go's default page (internal/service/chart.go defaultSearch); Go caps any limit at 50. */
const SEARCH_LIMIT = 20;

/**
 * The answer to one trimmed query. It is "fresh" only while that query is still what the box holds (Ruling 114 I1, the
 * Task 9 pattern): rows of an older query are never listed, counted, marked or added.
 */
interface Answer { q: string; rows: Instrument[]; failed: boolean }

/**
 * Sign-out (and the test reset) clears every mounted search box: an answer still in flight is then dropped, and an Add
 * that finishes afterwards comes back 'cancelled' from the store (Rulings 102 I2, 104 m1), so nothing is shown.
 */
const resetListeners = new Set<() => void>();
registerReset(() => resetListeners.forEach((f) => f()));

/**
 * FR-WATCH-004: inline search that adds to the active watchlist. A combobox with a listbox popup (WAI-ARIA 1.2
 * list-autocomplete pattern): focus stays in the box, arrows move the highlight, Enter or a click adds, Escape closes
 * the popup and a second Escape clears the box. Ranking is Go's (FR-WATCH-008); the order is shown as received.
 */
export function WatchlistSearch() {
  const list = useWatchlists(activeList);
  const id = useId();
  const listboxId = `${id}-results`;
  const optionId = (token: number) => `${id}-opt-${token}`;
  const [query, setQuery] = useState('');
  const [answer, setAnswer] = useState<Answer | null>(null);
  /** The last add, announced in the polite region until the query changes (Ruling 114 m2). */
  const [notice, setNotice] = useState('');
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  /** Bumped by every new query, clear, reset and unmount: only the latest search may write its answer. */
  const seq = useRef(0);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  /** Tokens whose Add is in flight: a repeated Enter or click does not send a second Add. */
  const adding = useRef(new Set<number>());

  const clear = () => {
    seq.current++;
    clearTimeout(timer.current);
    setQuery('');
    setAnswer(null);
    setNotice('');
    setOpen(false);
    setActive(0);
  };
  const clearRef = useRef(clear);
  clearRef.current = clear;

  useEffect(() => {
    const onReset = () => clearRef.current();
    const latest = seq;
    const pending = timer;
    resetListeners.add(onReset);
    return () => {
      resetListeners.delete(onReset);
      latest.current++; // an answer after unmount is dropped
      clearTimeout(pending.current);
    };
  }, []);

  const search = (value: string) => {
    setQuery(value);
    const q = value.trim();
    if (q === query.trim()) return; // same query (e.g. a trailing space): the answer stays fresh
    const my = ++seq.current;
    clearTimeout(timer.current);
    setNotice('');
    // Go answers [] for an empty query (searchInstruments skips the call): nothing is searched until the user types.
    if (q === '') return;
    timer.current = setTimeout(() => {
      searchInstruments(q, EXCHANGE_TAGS, SEARCH_LIMIT).then(
        (found) => {
          if (my !== seq.current) return;
          const rows = found.filter(isInScope); // NFR-15: never offer an option, MCX or currency row
          setAnswer({ q, rows, failed: false });
          setActive(0);
          setOpen(rows.length > 0);
        },
        () => {
          if (my !== seq.current) return;
          setAnswer({ q, rows: [], failed: true });
          setOpen(false);
        },
      );
    }, SEARCH_DEBOUNCE_MS);
  };

  const add = async (inst: Instrument) => {
    const target = activeList(useWatchlists.getState());
    if (!target || adding.current.has(inst.token)) return;
    adding.current.add(inst.token);
    try {
      useInstruments.getState().put([inst]); // rows look the instrument up by token later
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

  const fresh = answer !== null && answer.q === query.trim() ? answer : null;
  const results = fresh?.rows ?? [];
  const showList = open && results.length > 0;
  const highlighted = showList ? results[Math.min(active, results.length - 1)] : undefined;
  const highlightedId = highlighted ? optionId(highlighted.token) : undefined;

  useEffect(() => {
    // jsdom has no scrollIntoView; browsers do.
    if (highlightedId) document.getElementById(highlightedId)?.scrollIntoView?.({ block: 'nearest' });
  }, [highlightedId]);

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

  const message =
    notice ? notice
    : fresh?.failed ? copy.searchFailed
    : fresh ? (results.length > 0 ? copy.resultCount(results.length) : copy.noMatches)
    : '';
  // The count and the add notice are for screen readers only; "no matches" and a failure are shown too.
  const messageVisible = !notice && fresh !== null && (fresh.failed || results.length === 0);

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
        onChange={(e) => search(e.target.value)}
        onKeyDown={onKeyDown}
        onFocus={() => { if (results.length > 0) setOpen(true); }}
        onBlur={() => setOpen(false)}
      />
      <div role="status" aria-live="polite" className={messageVisible ? 'watchlist-search__message muted' : 'visually-hidden'}>
        {message}
      </div>
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
