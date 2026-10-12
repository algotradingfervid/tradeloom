import type { ReactNode } from 'react';
import { listCopy } from '../i18n/copy';
import { Button } from './Button';
import { Skeleton } from './Skeleton';
import './list.css';

export type ListStatus = 'loading' | 'error' | 'empty' | 'ready';

export interface ListBodyProps {
  status: ListStatus;
  error?: string | null;
  onRetry?: () => void;
  empty: { message: string; action?: { label: string; onSelect: () => void } };
  loadingLabel: string;
  children?: ReactNode;
}

function EmptyArt() {
  return (
    <svg className="list-state__art" width="96" height="72" viewBox="0 0 96 72" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" focusable="false">
      <rect x="8" y="12" width="80" height="52" rx="6" />
      <path d="M8 26h80M24 40h28M24 50h44" strokeLinecap="round" />
    </svg>
  );
}

export function ListBody({ status, error, onRetry, empty, loadingLabel, children }: ListBodyProps) {
  if (status === 'loading') return <Skeleton rows={6} label={loadingLabel} />;
  if (status === 'error') {
    return (
      <div className="list-state" role="alert">
        <p>{error || listCopy.genericError}</p>
        {onRetry && <Button onClick={onRetry}>{listCopy.retry}</Button>}
      </div>
    );
  }
  if (status === 'empty') {
    return (
      <div className="list-state">
        <EmptyArt />
        <p>{empty.message}</p>
        {empty.action && <Button variant="primary" onClick={empty.action.onSelect}>{empty.action.label}</Button>}
      </div>
    );
  }
  return <>{children}</>;
}
