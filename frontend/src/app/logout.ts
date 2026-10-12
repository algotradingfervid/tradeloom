import type { Backend } from '../api/backend';
import { parseBackendError, type BackendError } from '../api/errors';
import { logoutCopy } from '../i18n/copy';
import { pushToast } from '../store/toasts';

/**
 * FR-ACCT-004 log out, using Go's LogoutResult (Ruling 101). Go always completes the local logout and publishes
 * logged_out (the session gate then opens the login view); when Kite could not invalidate the token a gentle note
 * shows Go's message. A refusal (the token could not be deleted) is shown as an error toast and returned (null on
 * success) so a caller with its own form can also show it inline. Task 13's Log out and Settings → Kite connection
 * (Change credentials) call this.
 */
export async function logOut(backend: Backend): Promise<BackendError | null> {
  try {
    const result = await backend.session.logout();
    if (!result.remoteInvalidated) pushToast({ level: 'info', message: result.message || logoutCopy.notInvalidated });
    return null;
  } catch (e) {
    const failure = parseBackendError(e);
    pushToast({ level: 'error', message: failure.reason });
    return failure;
  }
}
