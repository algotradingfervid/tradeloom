import { useCallback, useEffect, useRef, useState } from 'react';
import { searchInstruments } from '../api/instruments';
import { instrumentSearchCopy } from '../i18n/copy';
import { registerReset } from '../store/reset';
import { EXCHANGE_TAGS, isInScope, type ExchangeTag } from './naming';
import type { Instrument } from './types';

export { instrumentSearchCopy };

/** The one search debounce of every instrument search box (FR-CHART-005, FR-WATCH-004/007). */
export const SEARCH_DEBOUNCE_MS = 120;

export interface InstrumentSearchOptions {
  /** Go clamps it to maxSearchLimit (50) and uses 20 for a limit <= 0 (internal/service/chart.go SearchInstruments). */
  limit: number;
  /** Tags sent to Instruments.Search and kept in the rows (default: all five in-scope tags). A new array with the same
   *  tags (an inline literal) does not start a new search. */
  tags?: readonly ExchangeTag[];
  /** Called after a reset (logout or user switch) cleared the answers; the owner clears its query here. */
  onReset?: () => void;
}

export interface InstrumentSearch {
  /** The trimmed query. */
  q: string;
  /** Rows to show: the latest answer (possibly for an older query while the next is pending); [] for an empty query or a failure. */
  rows: readonly Instrument[];
  /** true when `rows` answer `q`: only then may Enter pick a row. */
  fresh: boolean;
  /** The failure reason of the search for `q`, if it failed. */
  error: string | null;
  /** Text for the polite live region: the result count, the no-match text, or ''. */
  status: string;
  /** false once a reset happened after this answer arrived (read at call time, for event handlers). */
  isCurrent(): boolean;
}

// Late-answer guard across the app: a reset (logout, or the bridge's user switch) bumps the epoch, mounted searches
// drop their answers and anything still in flight (Rulings 102 I2, 107, 112).
let epoch = 0;
const listeners = new Set<() => void>();
registerReset(() => {
  epoch += 1;
  listeners.forEach((l) => l());
});

const NONE: readonly Instrument[] = [];
const message = (e: unknown) => (e instanceof Error ? e.message : String(e));

/**
 * Debounced Instruments.Search over the given tags (default: the five in-scope tags). An empty query is never sent
 * (Go answers []); only the answer to the latest query is kept; rows outside NFR-15 or outside `tags` are dropped;
 * unmounting clears the pending timer and silences the call in flight.
 *
 * Reset (logout or user switch): the answers and any call in flight are dropped. A consumer that keeps its query
 * across the reset gets that query searched again after the debounce; consumers should pass `onReset` and clear their
 * query there (as SymbolSearchDialog does), so nothing is sent for a session that has just ended.
 */
export function useInstrumentSearch(query: string, { limit, tags = EXCHANGE_TAGS, onReset }: InstrumentSearchOptions): InstrumentSearch {
  const q = query.trim();
  const tagsKey = (tags.length > 0 ? tags : EXCHANGE_TAGS).join(','); // [] means all, as in Go
  const [answer, setAnswer] = useState<{ q: string; epoch: number; rows: readonly Instrument[] } | null>(null);
  const [failure, setFailure] = useState<{ q: string; message: string } | null>(null);
  const [resets, setResets] = useState(0);
  const seq = useRef(0);
  const onResetRef = useRef(onReset);
  // A new search (other query, tags or limit) starts without the previous failure (adjusting state during render).
  const searchKey = `${q}|${tagsKey}|${limit}`;
  const [failureKey, setFailureKey] = useState(searchKey);
  if (failureKey !== searchKey) {
    setFailureKey(searchKey);
    setFailure(null);
  }
  useEffect(() => { onResetRef.current = onReset; });

  useEffect(() => {
    const listener = () => {
      seq.current += 1;
      setAnswer(null);
      setFailure(null);
      setResets((n) => n + 1);
      onResetRef.current?.();
    };
    listeners.add(listener);
    return () => {
      listeners.delete(listener);
      seq.current += 1;
    };
  }, []);

  useEffect(() => {
    const my = ++seq.current;
    if (q === '') return;
    const at = epoch;
    const live = () => my === seq.current && at === epoch;
    const wanted = tagsKey.split(',') as ExchangeTag[];
    const t = setTimeout(() => {
      searchInstruments(q, wanted, limit).then(
        (list) => {
          if (!live()) return;
          setAnswer({ q, epoch: at, rows: list.filter((i) => isInScope(i) && (wanted as readonly string[]).includes(i.tag)) });
          setFailure(null);
        },
        (e: unknown) => { if (live()) setFailure({ q, message: message(e) }); },
      );
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [q, tagsKey, limit, resets]);

  const isCurrent = useCallback(() => answer !== null && answer.epoch === epoch, [answer]);

  const error = failure !== null && failure.q === q ? failure.message : null;
  const rows = q === '' || error !== null || answer === null ? NONE : answer.rows;
  const fresh = q !== '' && error === null && answer?.q === q;
  let status = '';
  if (fresh) status = rows.length === 0 ? instrumentSearchCopy.noMatch : instrumentSearchCopy.count(rows.length);
  return { q, rows, fresh, error, status, isCurrent };
}
