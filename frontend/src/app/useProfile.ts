import { useCallback, useEffect } from 'react';
import { create } from 'zustand';
import type { Backend, Profile } from '../api/backend';
import { useBackend } from '../api/BackendContext';
import { parseBackendError } from '../api/errors';
import { profileMenuCopy } from '../i18n/copy';
import { registerReset } from '../store/reset';
import { useSessionStore } from '../store/session';
import { pushToast } from '../store/toasts';

/** The logged-in user id, or null when the session is not ready. */
const readyUser = (): string | null => {
  const s = useSessionStore.getState().session;
  return s?.state === 'ready' ? s.userId : null;
};

/**
 * One profile load per logged-in user, shared by every useProfile() caller (the top-bar menu and the Profile view;
 * Ruling 116 I1). `requestedFor`: the user a load was started for (set synchronously, so a second caller skips its
 * own); `failedFor`: the user whose last load failed; `loadingFor`: the user whose load is running.
 */
interface ProfileLoadState { requestedFor: string | null; failedFor: string | null; loadingFor: string | null }

const initial: ProfileLoadState = { requestedFor: null, failedFor: null, loadingFor: null };
const useProfileLoadStore = create<ProfileLoadState>()(() => ({ ...initial }));

/** The running load; concurrent callers for the same user get its promise instead of a second Profile call. */
let inFlight: { forUser: string; promise: Promise<void> } | null = null;

// Account-derived (user ids): reset on sign-out (default onLogout), like the profile itself.
registerReset(
  () => {
    useProfileLoadStore.setState(initial);
    inFlight = null;
  },
  { name: 'profileLoad', inspect: () => ({ ...useProfileLoadStore.getState(), inFlightFor: inFlight?.forUser ?? null }) },
);

/**
 * Loads `GET /user/profile` for the logged-in user (FR-SHELL-010, FR-ACCT-001). A response (or failure) that arrives
 * after the session left `ready` or moved to another user is dropped, toast included (Ruling 90 I1).
 */
export function loadProfile(backend: Backend): Promise<void> {
  const forUser = readyUser();
  if (forUser === null) return Promise.resolve();
  if (inFlight && inFlight.forUser === forUser) return inFlight.promise;
  useProfileLoadStore.setState({ requestedFor: forUser, failedFor: null, loadingFor: forUser });
  const entry = { forUser, promise: Promise.resolve() };
  inFlight = entry;
  entry.promise = (async () => {
    try {
      const p = await backend.session.profile();
      if (readyUser() === forUser && p.userId === forUser) useSessionStore.getState().setProfile(p);
    } catch (e) {
      if (readyUser() !== forUser) return;
      useProfileLoadStore.setState({ failedFor: forUser });
      pushToast({ level: 'error', message: profileMenuCopy.loadFailed(parseBackendError(e).reason) });
    } finally {
      // A reset (sign-out) or a newer load replaced this entry: leave their state alone.
      if (inFlight === entry) {
        inFlight = null;
        useProfileLoadStore.setState({ loadingFor: null });
      }
    }
  })();
  return entry.promise;
}

/**
 * The logged-in user's profile, loaded once per user. `failed` is true when the last load for the current user failed
 * (M1: the menu offers a retry instead of saying "Not logged in").
 */
export function useProfile(): { profile: Profile | null; failed: boolean; reload: () => Promise<void> } {
  const backend = useBackend();
  const ready = useSessionStore((s) => s.session?.state === 'ready');
  const userId = useSessionStore((s) => s.session?.userId ?? '');
  const profile = useSessionStore((s) => s.profile);
  const failedFor = useProfileLoadStore((s) => s.failedFor);

  const reload = useCallback(() => loadProfile(backend), [backend]);

  useEffect(() => {
    const { requestedFor } = useProfileLoadStore.getState();
    if (!ready) {
      // Expiry clears the profile (session.ts) without a sign-out reset; the next login fetches it again.
      if (requestedFor !== null) useProfileLoadStore.setState({ requestedFor: null });
      return;
    }
    if (profile || requestedFor === userId) return;
    void loadProfile(backend);
  }, [ready, userId, profile, backend]);

  return { profile, failed: ready && !profile && failedFor === userId, reload };
}

/** Whether a profile load for the logged-in user is running (the Profile view's Refresh shows it as busy). */
export function useProfileLoading(): boolean {
  const userId = useSessionStore((s) => (s.session?.state === 'ready' ? s.session.userId : null));
  return useProfileLoadStore((s) => userId !== null && s.loadingFor === userId);
}
