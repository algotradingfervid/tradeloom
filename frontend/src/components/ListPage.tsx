import { useId, useLayoutEffect, useRef, useState, type ReactNode, type RefObject } from 'react';
import { useNavigate } from 'react-router-dom';
import { listCopy } from '../i18n/copy';
import { useNavStore } from '../store/nav';
import { Icon } from './Icon';
import { IconButton } from './IconButton';
import './list.css';

export interface ListPageProps {
  title: string;
  description: string;
  onRefresh?: () => void;
  refreshing?: boolean;
  /** `matches`: rows left after filtering; 0 with a non-empty query announces "no matches" (Ruling 84 M5). */
  search?: { value: string; onChange: (v: string) => void; placeholder?: string; matches?: number };
  tabs?: ReactNode;
  children: ReactNode;
}

const TABBABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * WCAG 2.1.1 (axe scrollable-region-focusable): a body that scrolls but holds nothing tabbable (legal text, a long
 * help article) cannot be scrolled from the keyboard, so it becomes a focusable region. A body with tabbable content
 * gets no extra tab stop: tabbing into the content scrolls it.
 */
function useNeedsScrollFocus(ref: RefObject<HTMLElement | null>): boolean {
  const [needs, setNeeds] = useState(false);
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const update = () => {
      const scrolls = el.scrollHeight > el.clientHeight;
      const hasTabbable = [...el.querySelectorAll(TABBABLE)].some((c) => c !== el);
      setNeeds(scrolls && !hasTabbable);
    };
    update();
    const resize = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(update);
    resize?.observe(el);
    const mutations = new MutationObserver(update);
    mutations.observe(el, { childList: true, subtree: true, characterData: true });
    return () => { resize?.disconnect(); mutations.disconnect(); };
  }, [ref]);
  return needs;
}

export function ListPage({ title, description, onRefresh, refreshing, search, tabs, children }: ListPageProps) {
  const navigate = useNavigate();
  const titleId = useId();
  const bodyRef = useRef<HTMLDivElement>(null);
  const scrollFocus = useNeedsScrollFocus(bodyRef);
  const goBack = () => {
    if (useNavStore.getState().depth > 1) void navigate(-1);
    else void navigate('/home');
  };
  return (
    <div className="list-page">
      <header className="list-page__header">
        <IconButton label={listCopy.back} icon="back" onClick={goBack} />
        <div className="list-page__titles">
          <h1 id={titleId}>{title}</h1>
          <p className="muted">{description}</p>
        </div>
        <div className="list-page__tools">
          {onRefresh && (
            // aria-disabled (not disabled) so focus is not stranded on the button while a refresh runs (Ruling 22).
            <IconButton
              label={listCopy.refresh(title)}
              icon="refresh"
              aria-disabled={refreshing || undefined}
              aria-busy={refreshing || undefined}
              onClick={() => { if (!refreshing) onRefresh(); }}
            />
          )}
          {search && (
            <label className="search-box">
              <Icon name="search" size={16} />
              <input
                type="search"
                aria-label={listCopy.search(title)}
                placeholder={search.placeholder ?? listCopy.searchPlaceholder}
                value={search.value}
                onChange={(e) => search.onChange(e.target.value)}
              />
            </label>
          )}
        </div>
      </header>
      {tabs}
      {search && (
        <p className="list-page__matches muted" role="status" aria-live="polite">
          {search.matches === 0 && search.value.trim() !== '' ? listCopy.noMatches(search.value.trim()) : null}
        </p>
      )}
      <div
        ref={bodyRef}
        className="list-page__body"
        {...(scrollFocus ? { tabIndex: 0, role: 'region', 'aria-labelledby': titleId } : {})}
      >
        {children}
      </div>
    </div>
  );
}
