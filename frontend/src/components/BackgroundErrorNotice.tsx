import { listCopy } from '../i18n/copy';
import { Button } from './Button';
import './list.css';

/**
 * Ruling 103: a failed background re-read keeps the shown data; this lasting inline notice says so and offers Retry
 * (the same design as the Home holdings card's notice). Renders nothing while `error` is null.
 */
export function BackgroundErrorNotice({ error, message, onRetry, className = 'list-notice' }: {
  error: string | null;
  message: (reason: string) => string;
  onRetry: () => void;
  className?: string;
}) {
  if (error === null) return null;
  return (
    <div className={className} role="status">
      <p>{message(error)}</p>
      <Button onClick={onRetry}>{listCopy.retry}</Button>
    </div>
  );
}
