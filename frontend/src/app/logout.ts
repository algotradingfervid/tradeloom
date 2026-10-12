import type { Backend } from '../api/backend';
import { parseBackendError } from '../api/errors';
import { logoutCopy } from '../i18n/copy';
import { pushToast } from '../store/toasts';

/**
 * FR-ACCT-004 log out, using Go's LogoutResult (Ruling 101). Go always completes the local logout and publishes
 * logged_out (the session gate then opens the login view); when Kite could not invalidate the token a gentle note
 * shows Go's message. A refusal (the token could not be deleted) is shown as an error. Task 13's Log out calls this.
 */
export async function logOut(backend: Backend): Promise<boolean> {
  try {
    const result = await backend.session.logout();
    if (!result.remoteInvalidated) pushToast({ level: 'info', message: result.message || logoutCopy.notInvalidated });
    return true;
  } catch (e) {
    pushToast({ level: 'error', message: parseBackendError(e).reason });
    return false;
  }
}
