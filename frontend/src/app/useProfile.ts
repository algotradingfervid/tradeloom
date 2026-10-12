import { useCallback, useEffect, useRef, useState } from 'react';
import type { Profile } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { profileMenuCopy } from '../i18n/copy';
import { useSessionStore } from '../store/session';
import { pushToast } from '../store/toasts';

/** The logged-in user id, or null when the session is not ready. */
const readyUser = (): string | null => {
  const s = useSessionStore.getState().session;
  return s?.state === 'ready' ? s.userId : null;
};

/**
 * Loads `GET /user/profile` once per logged-in user (FR-SHELL-010, FR-ACCT-001). A response (or failure) that arrives
 * after the session left `ready` or moved to another user is dropped (Ruling 90 I1). `failed` is true when the last
 * load for the current user failed (M1: the menu offers a retry instead of saying "Not logged in").
 */
export function useProfile(): { profile: Profile | null; failed: boolean; reload: () => Promise<void> } {
  const backend = useBackend();
  const ready = useSessionStore((s) => s.session?.state === 'ready');
  const userId = useSessionStore((s) => s.session?.userId ?? '');
  const profile = useSessionStore((s) => s.profile);
  const requestedFor = useRef<string | null>(null);
  const [failedFor, setFailedFor] = useState<string | null>(null);

  const reload = useCallback(async () => {
    const forUser = readyUser();
    if (forUser === null) return;
    setFailedFor(null);
    try {
      const p = await backend.session.profile();
      if (readyUser() === forUser && p.userId === forUser) useSessionStore.getState().setProfile(p);
    } catch (e) {
      if (readyUser() !== forUser) return;
      setFailedFor(forUser);
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

  return { profile, failed: ready && !profile && failedFor === userId, reload };
}
