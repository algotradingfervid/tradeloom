import { useCallback, useEffect, useRef } from 'react';
import type { Profile } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { profileMenuCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';
import { pushToast } from '../store/toasts';

/** Loads `GET /user/profile` once per logged-in user (FR-SHELL-010, FR-ACCT-001). */
export function useProfile(): { profile: Profile | null; reload: () => Promise<void> } {
  const backend = useBackend();
  const ready = useSessionStore((s) => s.session?.state === 'ready');
  const userId = useSessionStore((s) => s.session?.userId ?? '');
  const profile = useSessionStore((s) => s.profile);
  const requestedFor = useRef<string | null>(null);

  const reload = useCallback(async () => {
    try {
      useSessionStore.getState().setProfile(await backend.session.profile());
    } catch (e) {
      pushToast({ level: 'error', message: profileMenuCopy.loadFailed(parseBackendError(e).reason) });
    }
  }, [backend]);

  useEffect(() => {
    if (!ready) {
      requestedFor.current = null; // sign-out clears the profile (session.ts); the next login fetches it again
      return;
    }
    if (profile || requestedFor.current === userId) return;
    requestedFor.current = userId;
    void reload();
  }, [ready, userId, profile, reload]);

  return { profile, reload };
}
