import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react';
import { parseBackendError } from '../../api/errors';
import { watchlistPanelCopy as copy } from '../../i18n/copy';
import { pushToast } from '../../store/toasts';
import { activeList, useWatchlists } from '../../store/watchlistsStore';
import { nextWatchlistName } from './names';
import './watchlists.css';

const reason = (e: unknown): string => (e instanceof Error ? e.message : parseBackendError(e).reason);

/** The id of a watchlist's tab, for the owner's tabpanel `aria-labelledby`. */
export const watchlistTabId = (prefix: string, listId: string) => `${prefix}-tab-${listId}`;

/**
 * FR-WATCH-001/002: the watchlist tabs, "New watchlist +", rename (double-click, F2 or the context menu) and the inline
 * name errors. WAI-ARIA tabs with automatic activation and roving focus: only the active tab is tabbable,
 * ArrowLeft/ArrowRight wrap, Home/End jump; moving to a tab activates it (Go persists the active list). The name fields
 * sit below the tablist, not inside it (a tablist owns only tabs). `panelId` names the owner's tabpanel.
 */
export function WatchlistTabs({ panelId, idPrefix }: { panelId?: string; idPrefix?: string }) {
  const lists = useWatchlists((s) => s.lists);
  const activeId = useWatchlists((s) => activeList(s)?.id);
  const ownId = useId();
  const prefix = idPrefix ?? ownId;
  const errorId = `${ownId}-error`;
  const menuId = `${ownId}-menu`;
  const [creating, setCreating] = useState<string | null>(null);
  const [renaming, setRenaming] = useState<{ id: string; value: string } | null>(null);
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const tabs = useRef(new Map<string, HTMLButtonElement>());
  const newButton = useRef<HTMLButtonElement>(null);
  const menu = useRef<HTMLUListElement>(null);
  /** A tab to focus once it is rendered ('active' = whichever tab is active then). */
  const pendingFocus = useRef<string | null>(null);

  // A list that disappeared (deleted elsewhere, or a reset) takes its rename field and menu with it.
  const renamingList = renaming ? lists.find((l) => l.id === renaming.id) : undefined;
  const menuList = menuFor ? lists.find((l) => l.id === menuFor) : undefined;

  useEffect(() => {
    const want = pendingFocus.current === 'active' ? activeId : pendingFocus.current;
    const el = want ? tabs.current.get(want) : undefined;
    if (el) {
      pendingFocus.current = null;
      el.focus();
    }
  });

  useEffect(() => {
    if (!menuList) return;
    menu.current?.querySelector<HTMLElement>('[role="menuitem"]')?.focus();
    const onDown = (e: MouseEvent) => {
      if (!menu.current?.contains(e.target as Node)) setMenuFor(null);
    };
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [menuList]);

  const activate = (id: string) => {
    if (id === activeId) return;
    useWatchlists.getState().setActive(id).catch((e: unknown) => pushToast({ level: 'error', message: copy.switchFailed(reason(e)) }));
  };

  const onTabKeyDown = (e: KeyboardEvent<HTMLButtonElement>, i: number, id: string) => {
    if (e.key === 'F2') {
      e.preventDefault();
      startRename(id);
      return;
    }
    if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
    const n = lists.length;
    const moves: Record<string, number> = { ArrowRight: (i + 1) % n, ArrowLeft: (i - 1 + n) % n, Home: 0, End: n - 1 };
    const j = moves[e.key];
    if (j === undefined) return;
    e.preventDefault();
    const target = lists[j];
    tabs.current.get(target.id)?.focus();
    activate(target.id);
  };

  const startCreate = () => {
    setRenaming(null);
    setMenuFor(null);
    setError(null);
    setCreating(nextWatchlistName(lists.map((l) => l.name)));
  };
  const startRename = (id: string) => {
    const l = lists.find((x) => x.id === id);
    setMenuFor(null);
    setCreating(null);
    setError(null);
    if (l) setRenaming({ id: l.id, value: l.name });
  };
  const closeRename = (id: string) => {
    setRenaming(null);
    setError(null);
    pendingFocus.current = id;
  };
  const closeCreate = () => {
    setCreating(null);
    setError(null);
    newButton.current?.focus();
  };

  /** One submit at a time; the field stays focused and editable (aria-busy) while Go answers. */
  const run = async (submit: () => Promise<string | null>, done: () => void) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    try {
      const err = await submit();
      if (err) setError(err);
      else done();
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };
  const submitCreate = () =>
    run(() => useWatchlists.getState().create(creating ?? ''), () => {
      setCreating(null);
      setError(null);
      pendingFocus.current = 'active'; // Go's Create activates the new list (Ruling 87)
    });
  const submitRename = () => {
    if (!renamingList || !renaming) return Promise.resolve();
    const { id, value } = renaming;
    // An unchanged name closes the field without asking Go.
    if (value.trim() === renamingList.name) {
      closeRename(id);
      return Promise.resolve();
    }
    return run(() => useWatchlists.getState().rename(id, value), () => closeRename(id));
  };
  const fieldKeys = (submit: () => Promise<void>, cancel: () => void) => (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.nativeEvent.isComposing) return; // IME composition (Ruling 112 minor)
    if (e.key === 'Enter' && !(e.altKey || e.ctrlKey || e.metaKey || e.shiftKey)) {
      e.preventDefault();
      void submit();
    } else if (e.key === 'Escape') {
      e.preventDefault();
      e.stopPropagation();
      cancel();
    }
  };
  const fieldProps = {
    className: 'field__input watchlist-name-field',
    autoFocus: true,
    autoComplete: 'off',
    spellCheck: false,
    'aria-busy': busy || undefined,
    'aria-invalid': error ? true : undefined,
    'aria-describedby': error ? errorId : undefined,
  } as const;

  return (
    <div className="watchlist-tabs-wrap">
      <div className="watchlist-panel__header">
        <h2>{copy.heading}</h2>
        <button ref={newButton} type="button" className="btn btn--ghost watchlist-new" onClick={startCreate}>{copy.newList}</button>
      </div>
      <div role="tablist" aria-label={copy.tablist} className="watchlist-tabs">
        {lists.map((l, i) => {
          const selected = l.id === activeId;
          return (
            <button
              key={l.id}
              ref={(el) => { if (el) tabs.current.set(l.id, el); else tabs.current.delete(l.id); }}
              id={watchlistTabId(prefix, l.id)}
              type="button"
              role="tab"
              aria-selected={selected}
              aria-controls={panelId}
              aria-haspopup="menu"
              aria-keyshortcuts="F2"
              tabIndex={selected ? 0 : -1}
              className="watchlist-tab"
              title={copy.renameHint}
              onClick={() => activate(l.id)}
              onDoubleClick={() => startRename(l.id)}
              onContextMenu={(e) => { e.preventDefault(); setMenuFor(l.id); }}
              onKeyDown={(e) => onTabKeyDown(e, i, l.id)}
            >
              {l.name}
            </button>
          );
        })}
      </div>
      {menuList && (
        <ul
          id={menuId}
          ref={menu}
          role="menu"
          aria-label={copy.menu(menuList.name)}
          className="watchlist-menu"
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault();
              e.stopPropagation();
              setMenuFor(null);
              tabs.current.get(menuList.id)?.focus();
            } else if (e.key === 'Tab') setMenuFor(null);
          }}
        >
          <li role="none">
            <button type="button" role="menuitem" tabIndex={-1} className="menu__item" onClick={() => startRename(menuList.id)}>{copy.rename}</button>
          </li>
        </ul>
      )}
      {renamingList && renaming && (
        <input {...fieldProps} aria-label={copy.renameField} value={renaming.value}
          onChange={(e) => setRenaming({ id: renamingList.id, value: e.target.value })}
          onKeyDown={fieldKeys(submitRename, () => closeRename(renamingList.id))} />
      )}
      {creating !== null && (
        <input {...fieldProps} aria-label={copy.newName} value={creating}
          onChange={(e) => setCreating(e.target.value)} onKeyDown={fieldKeys(submitCreate, closeCreate)} />
      )}
      {error && <p id={errorId} role="alert" className="watchlist-error">{error}</p>}
    </div>
  );
}
