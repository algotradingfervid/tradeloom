import { useId, useRef, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { useBackend } from '../../api/BackendContext';
import { parseBackendError } from '../../api/errors';
import { Button } from '../../components/Button';
import { TextField } from '../../components/TextField';
import { loginCopy as copy } from '../../i18n/copy';
import { useSessionStore } from '../../store/session';
import { pushToast } from '../../store/toasts';
import { adoptReturnedStatus } from './adoptStatus';
import { OnboardingFrame } from './OnboardingFrame';

/** Go kite.CodeListenerTimeout: the login window closed; Go's message asks the user to paste the address. */
const LISTENER_TIMEOUT = 'listener_timeout';
/**
 * Go codes whose message only repeats that the session ended (kite.CodeSessionExpired, CodeTokenRejected): the view
 * already says so (copy.expired), so Go's text is not shown a second time (Ruling 108 I2).
 */
const EXPIRY_CODES: ReadonlySet<string> = new Set(['session_expired', 'token_rejected']);

export function LoginView() {
  const backend = useBackend();
  const session = useSessionStore((s) => s.session);
  const [listenerFailed, setListenerFailed] = useState(false);
  const [redirectUrl, setRedirectUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showPaste, setShowPaste] = useState(false);
  // The pasted redirect URL carries request_token (Ruling 101): component-local only, cleared after every submit and
  // gone on unmount; never echoed back in a message, toast, store, web storage, the URL or a log.
  const [pasted, setPasted] = useState('');
  const [pasteError, setPasteError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const pasteRef = useRef<HTMLInputElement>(null);
  const cancelRef = useRef<HTMLButtonElement>(null);
  const pasteFormId = useId();

  // Go ends a login with an error code on the session (listener timeout, refused exchange): open the paste field
  // once when the timeout arrives (adjusting state while rendering, not in an effect).
  const errorCode = session?.errorCode ?? '';
  // Starts empty so a session that already carries a listener timeout opens the paste field on arrival (M9).
  const [seenCode, setSeenCode] = useState('');
  if (errorCode !== seenCode) {
    setSeenCode(errorCode);
    if (errorCode === LISTENER_TIMEOUT) setShowPaste(true);
  }

  // Waiting follows Go's state: a timeout, cancel or success ends it whoever caused it.
  const waiting = session?.state === 'logging_in' && !listenerFailed;
  const goError =
    session && session.state !== 'logging_in' && session.state !== 'expired' && !EXPIRY_CODES.has(session.errorCode) && session.errorMessage
      ? session.errorMessage
      : null;
  const guidanceUrl = redirectUrl || session?.credentials.redirectUrl || '';

  // Login() opens the system browser and returns LoginStart; success arrives as a "session.state" event (A3).
  const login = async () => {
    setError(null);
    setListenerFailed(false);
    try {
      const start = await backend.session.login();
      setRedirectUrl(start.redirectUrl || null);
      if (start.listenerOk) {
        // Log in is disabled while waiting (M8): keep focus on a live control (Ruling 22).
        requestAnimationFrame(() => cancelRef.current?.focus());
        return;
      }
      // The loopback listener could not start (port busy): the browser still opens, the user pastes the redirect.
      setListenerFailed(true);
      setError(start.listenerError);
      setShowPaste(true);
    } catch (e) {
      const reason = parseBackendError(e).reason;
      setError(reason);
      setShowPaste(true);
      pushToast({ level: 'error', message: reason });
    }
  };

  const cancel = async () => {
    try {
      adoptReturnedStatus(await backend.session.cancelLogin());
    } catch (e) {
      pushToast({ level: 'error', message: parseBackendError(e).reason });
    }
  };

  const submitPaste = async (e: FormEvent) => {
    e.preventDefault();
    if (busy) return;
    const value = pasted.trim();
    if (!value) {
      setPasteError(copy.pasteRequired);
      pasteRef.current?.focus(); // Ruling 108 I1
      return;
    }
    setPasteError(null);
    setBusy(true);
    try {
      adoptReturnedStatus(await backend.session.completeLogin(value));
    } catch (err) {
      const reason = parseBackendError(err).reason;
      setPasteError(reason);
      pushToast({ level: 'error', message: reason });
    } finally {
      setPasted('');
      setBusy(false);
    }
  };

  return (
    <OnboardingFrame title={copy.title}>
      {session?.state === 'expired' && <p className="onboarding__notice" role="status">{copy.expired}</p>}
      <p>{copy.intro}</p>
      {guidanceUrl && <p className="muted">{copy.redirectGuidance(guidanceUrl)}</p>}
      <Link to="/settings/kite">{copy.settingsLink}</Link>
      {/* M8: no second login while one waits for the browser; Cancel stays available. */}
      <Button variant="primary" onClick={login} disabled={waiting}>{copy.button}</Button>
      {waiting && (
        <div className="row">
          <p role="status">{copy.waiting}</p>
          <Button ref={cancelRef} onClick={cancel}>{copy.cancel}</Button>
        </div>
      )}
      {error && <p className="field__error" role="alert">{error}</p>}
      {goError && goError !== error && goError !== pasteError && <p className="field__error" role="alert">{goError}</p>}
      <button type="button" className="link-button" aria-expanded={showPaste} aria-controls={pasteFormId} onClick={() => setShowPaste((v) => !v)}>
        {copy.pasteToggle}
      </button>
      {showPaste && (
        <form id={pasteFormId} onSubmit={submitPaste} noValidate>
          <TextField ref={pasteRef} label={copy.pasteLabel} value={pasted} onChange={(e) => setPasted(e.target.value)} error={pasteError} hint={copy.pasteHint} autoComplete="off" spellCheck={false} />
          <Button type="submit" disabled={busy}>{copy.pasteSubmit}</Button>
        </form>
      )}
    </OnboardingFrame>
  );
}
