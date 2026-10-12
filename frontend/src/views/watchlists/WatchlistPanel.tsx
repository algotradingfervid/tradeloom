import { useEffect, useId, useRef, useState } from 'react';
import type { WatchlistItem } from '../../api/backend';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { IconButton } from '../../components/IconButton';
import { watchlistPanelCopy as copy } from '../../i18n/copy';
import { registerReset } from '../../store/reset';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists, type Watchlist } from '../../store/watchlistsStore';
import { InstrumentPicker } from './InstrumentPicker';
import { WatchlistRows } from './WatchlistRows';
import { WatchlistSearch } from './WatchlistSearch';
import { watchlistTabId, WatchlistTabs } from './WatchlistTabs';
import './watchlists.css';

const reason = (e: unknown): string => (e instanceof Error ? e.message : parseBackendError(e).reason);

/** Counts resets (sign-out or user switch): a picker batch that finishes after one announces nothing (Ruling 118). */
let resets = 0;
registerReset(() => { resets += 1; }, { name: 'watchlistPanelResets', inspect: () => ({ resets }) });

/**
 * FR-WATCH-001/002/003: the watchlist panel — tabs, inline search, delete (with confirmation) and the active list's
 * rows. The event bridge loads the store on →ready and at bootstrap (Ruling 102 I3); the mount effect is only a
 * fallback while the store is still idle. Busy controls stay focusable (aria-disabled, P0-4 Ruling 84 M6/M7).
 */
export function WatchlistPanel({ onChartItem }: { onChartItem(item: WatchlistItem): void }) {
  const status = useWatchlists((s) => s.status);
  const error = useWatchlists((s) => s.error);
  const active = useWatchlists(activeList);
  const prefix = useId();
  const panelId = `${prefix}-panel`;
  const [confirming, setConfirming] = useState<Watchlist | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [retrying, setRetrying] = useState(false);
  /** The list the picker was opened for (its name titles the picker and the announcement). */
  const [pickerFor, setPickerFor] = useState<Watchlist | null>(null);
  const [notice, setNotice] = useState('');
  // The last load error stays on screen while Retry runs (load() clears the store's error); adjusted during render.
  const [lastError, setLastError] = useState(error);
  if (error && error !== lastError) setLastError(error);
  const deletingRef = useRef(false);
  const retryingRef = useRef(false);

  useEffect(() => {
    if (useWatchlists.getState().status === 'idle') void useWatchlists.getState().load();
  }, []);

  const retry = () => {
    if (retryingRef.current) return;
    retryingRef.current = true;
    setRetrying(true);
    void useWatchlists.getState().load().finally(() => {
      retryingRef.current = false;
      setRetrying(false);
    });
  };

  const remove = (list: Watchlist) => {
    setConfirming(null);
    if (deletingRef.current) return;
    deletingRef.current = true;
    setDeleting(true);
    // Go picks the next active list; deleting the only list leaves a fresh "Watchlist 1".
    useWatchlists.getState().deleteList(list.id)
      .catch((e: unknown) => pushToast({ level: 'error', message: copy.deleteFailed(list.name, reason(e)) }))
      .finally(() => {
        deletingRef.current = false;
        setDeleting(false);
      });
  };

  // Done in the picker: append the picked instruments to the active list (Task 17 Step 3, Ruling 118 notes).
  // addItems rejects ("No watchlist", unknown_instrument) — never voided; after a reset it resolves quietly.
  const addPicked = (list: Watchlist, tokens: number[]) => {
    setPickerFor(null);
    if (tokens.length === 0) return;
    const at = resets;
    setNotice('');
    useWatchlists.getState().addItems(tokens)
      .then((n) => { if (n > 0 && resets === at) setNotice(copy.addedMany(n, list.name)); })
      .catch((e: unknown) => { if (resets === at) pushToast({ level: 'error', message: copy.addFailed(list.name, reason(e)) }); });
  };

  const showError = status === 'error' || retrying;

  return (
    <section className="watchlist-panel" aria-label={copy.region}>
      <WatchlistTabs panelId={active ? panelId : undefined} idPrefix={prefix} />
      <div className="watchlist-panel__tools">
        <WatchlistSearch />
        {active && <IconButton label={copy.addInstruments} tooltip={copy.addTitle(active.name)} icon="plus" onClick={() => setPickerFor(active)} />}
        {active && (
          <IconButton
            label={copy.deleteLabel(active.name)}
            icon="trash"
            aria-disabled={deleting || undefined}
            aria-busy={deleting || undefined}
            onClick={() => { if (!deletingRef.current) setConfirming(active); }}
          />
        )}
      </div>
      {showError && (
        <p role="alert" className="watchlist-error">
          <span>{copy.loadFailed(error ?? lastError ?? '')}</span>
          <Button aria-disabled={retrying || undefined} aria-busy={retrying || undefined} onClick={retry}>{copy.retry}</Button>
        </p>
      )}
      {status === 'loading' && !retrying && !active && <p role="status" className="watchlist-empty">{copy.loading}</p>}
      {active && (
        <div role="tabpanel" id={panelId} aria-labelledby={watchlistTabId(prefix, active.id)} className="watchlist-panel__body">
          <WatchlistRows list={active} onChartItem={onChartItem} />
        </div>
      )}
      <p role="status" className="visually-hidden">{notice}</p>
      <InstrumentPicker
        open={pickerFor !== null}
        title={pickerFor ? copy.addTitle(pickerFor.name) : undefined}
        onClose={() => setPickerFor(null)}
        onDone={(sel) => { if (pickerFor) addPicked(pickerFor, sel.map((i) => i.token)); }}
      />
      <ConfirmDialog
        open={confirming !== null}
        title={copy.deleteTitle}
        tone="danger"
        message={confirming ? copy.deleteMessage(confirming.name, confirming.items.length) : ''}
        confirmLabel={copy.deleteConfirm}
        onConfirm={() => { if (confirming) remove(confirming); }}
        onCancel={() => setConfirming(null)}
      />
    </section>
  );
}
