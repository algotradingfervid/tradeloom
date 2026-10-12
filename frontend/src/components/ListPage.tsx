import type { ReactNode } from 'react';
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

export function ListPage({ title, description, onRefresh, refreshing, search, tabs, children }: ListPageProps) {
  const navigate = useNavigate();
  const goBack = () => {
    if (useNavStore.getState().depth > 1) void navigate(-1);
    else void navigate('/home');
  };
  return (
    <div className="list-page">
      <header className="list-page__header">
        <IconButton label={listCopy.back} icon="back" onClick={goBack} />
        <div className="list-page__titles">
          <h1>{title}</h1>
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
      <div className="list-page__body">{children}</div>
    </div>
  );
}
