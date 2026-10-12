import { create } from 'zustand';
import type { Profile, SessionStatus } from '../api/backend';
import { registerReset } from './reset';

interface SessionStore {
  session: SessionStatus | null;
  profile: Profile | null;
  setSession(s: SessionStatus): void;
  setProfile(p: Profile | null): void;
}

const initial = { session: null, profile: null };

export const useSessionStore = create<SessionStore>()((set) => ({
  ...initial,
  setSession: (session) =>
    set((st) => ({
      session,
      profile: st.profile && session.state === 'ready' && st.profile.userId === session.userId ? st.profile : null,
    })),
  setProfile: (profile) => set({ profile }),
}));

registerReset(() => useSessionStore.setState(initial), { onLogout: false, name: 'session', inspect: () => useSessionStore.getState() });
