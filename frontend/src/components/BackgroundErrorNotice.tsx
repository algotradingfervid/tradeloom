import { useEffect, useRef, useState } from 'react';
import { listCopy } from '../i18n/copy';
import { Button } from './Button';
import './list.css';

/**
 * A failed read keeps the shown data (Rulings 103, 105); this lasting inline notice says so and offers Retry (the
 * same design as the Home holdings card's notice).
 * - The role="status" live region is always mounted and only its contents toggle, so screen readers announce it.
 * - `seq` moves on every failure: the message is re-keyed so an identical repeated failure is announced again.
 * - Retry is aria-disabled (and ignores clicks) until its promise settles.
 */
export function BackgroundErrorNotice({ error, seq, message, onRetry, className = 'list-notice' }: {
  error: string | null;
  seq: number;
  message: (reason: string) => string;
  onRetry: () => Promise<unknown>;
  className?: string;
}) {
  const [pending, setPending] = useState(false);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);
  const retry = () => {
    if (pending) return;
    setPending(true);
    void onRetry().finally(() => {
      if (mounted.current) setPending(false);
    });
  };
  return (
    <div className="notice-region" role="status">
      {error !== null && (
        <div className={className}>
          <p key={seq}>{message(error)}</p>
          <Button onClick={retry} aria-disabled={pending || undefined} aria-busy={pending || undefined}>{listCopy.retry}</Button>
        </div>
      )}
    </div>
  );
}
