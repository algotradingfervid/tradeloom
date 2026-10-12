import { useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import type { ClockOffset } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { Button } from '../components/Button';
import { Dialog } from '../components/Dialog';
import { Tooltip } from '../components/Tooltip';
import { statusCopy } from '../i18n/copy';
import { formatSigned } from '../i18n/format';
import { useSessionStore } from '../store/session';
import { useStatusStore } from '../store/status';
import { pushToast } from '../store/toasts';
import { describeStatus } from './describeStatus';
import { useNow } from './useNow';

/**
 * P0-2 sets `warn` when |offset| > 1 s; the toast is shown once per app start. A store subscription (not a render
 * effect) sees every `clock.offset`, so the first warning offset is reported even when events arrive in one batch.
 */
function useClockWarning(): void {
  const warned = useRef(false);
  useEffect(() => {
    const check = (clock: ClockOffset | null) => {
      if (!clock || warned.current || !clock.warn) return;
      warned.current = true;
      pushToast({ level: 'warning', message: statusCopy.clockWarning(formatSigned(clock.millis / 1000, 1)) });
    };
    check(useStatusStore.getState().clock);
    return useStatusStore.subscribe((s, prev) => {
      if (s.clock !== prev.clock) check(s.clock);
    });
  }, []);
}

export function StatusIndicator() {
  const backend = useBackend();
  const navigate = useNavigate();
  const feed = useStatusStore((s) => s.feed);
  const market = useStatusStore((s) => s.market);
  const clock = useStatusStore((s) => s.clock);
  const session = useSessionStore((s) => s.session);
  const now = useNow();
  const view = describeStatus({ feed, session, market, clock, now });
  const [open, setOpen] = useState(false);
  useClockWarning();

  const reconnect = () => {
    if (!view.canReconnect) return;
    // Go may refuse (not_logged_in / unavailable): show its reason, never a success message.
    backend.session.reconnectFeed().then(
      () => pushToast({ level: 'info', message: statusCopy.reconnectRequested }),
      (e: unknown) => pushToast({ level: 'error', message: parseBackendError(e).reason }),
    );
  };

  return (
    <>
      <Tooltip label={view.lines.join('\n')}>
        <button
          type="button"
          className={`status status--${view.tone}`}
          aria-label={statusCopy.button(view.label, view.marketClosed)}
          onClick={() => setOpen(true)}
        >
          <span className="status__dot" aria-hidden="true" />
          <span className="status__label">{view.label}</span>
          {view.marketClosed && <span className="status__closed">{statusCopy.marketClosed}</span>}
        </button>
      </Tooltip>
      <Dialog
        open={open}
        title={statusCopy.panelTitle}
        size="sm"
        onClose={() => setOpen(false)}
        footer={
          <>
            {/* aria-disabled keeps the button focusable so focus survives state changes (review focus #3) */}
            <Button onClick={reconnect} aria-disabled={!view.canReconnect}>{statusCopy.reconnect}</Button>
            {view.canLogIn && (
              <Button variant="primary" onClick={() => { setOpen(false); void navigate('/login'); }}>{statusCopy.logIn}</Button>
            )}
          </>
        }
      >
        <ul className="status-panel">
          {view.lines.map((line) => <li key={line}>{line}</li>)}
        </ul>
        {view.state === 'reconnecting' && <p className="status-panel__note">{statusCopy.reconnectWaiting}</p>}
      </Dialog>
    </>
  );
}
