import type { SessionStatus } from '../../api/backend';
import { useSessionStore } from '../../store/session';

/**
 * Applies a SessionStatus a Session binding returned. Go publishes "session.state" on every state transition (the
 * event bridge applies those, with its user-switch reset, Ruling 98), but not for every change inside a state: e.g.
 * RefuseSecretStorage only changes `secrets` and publishes nothing. So the returned status is adopted only when it
 * keeps the current state and user; anything else is left to the event.
 */
export function adoptReturnedStatus(next: SessionStatus): void {
  const store = useSessionStore.getState();
  const cur = store.session;
  if (cur && cur.state === next.state && cur.userId === next.userId) store.setSession(next);
}
