import type { SessionStatus } from '../api/backend';

export const ONBOARDING_ROUTES: readonly string[] = ['/setup', '/login', '/risk'];
const ALWAYS_ALLOWED = [/^\/legal\/[a-z-]+$/, /^\/help(\/[a-z0-9-]+)?$/];
/**
 * Settings stay reachable with credentials but no session (merge with Task 14, Ruling 97): a busy redirect port or
 * wrong credentials can only be fixed in Settings → Kite connection, and Ruling 97 shows Change disabled with an
 * explanation while a login runs. Not while locked or without credentials: setup comes first there.
 */
const SETTINGS = /^\/settings(\/[a-z-]+)?$/;

/** Where the session requires the user to be, or null to stay (FR-ACCT-005/006, FR-SHELL-020). */
export function gateTarget(session: SessionStatus, path: string): string | null {
  if (ALWAYS_ALLOWED.some((r) => r.test(path))) return null;
  switch (session.state) {
    case 'no_credentials':
    case 'locked':
      return path === '/setup' ? null : '/setup';
    case 'logged_out':
    case 'logging_in':
      return path === '/login' || SETTINGS.test(path) ? null : '/login';
    case 'expired':
      return null;
    case 'ready':
      if (session.riskAckRequired) return path === '/risk' ? null : '/risk';
      return ONBOARDING_ROUTES.includes(path) ? '/' : null;
  }
}
