import { describe, expect, it } from 'vitest';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend } from '../api/mock/index';
import { useToastStore } from '../store/toasts';
import { logOut } from './logout';

// Go kite.msgNotInvalidated (internal/kite/logout.go).
const NOT_INVALIDATED = 'Logged out of Tradeloom. Kite could not be reached to invalidate the token; it stops working at 06:00 IST.';
const toasts = () => useToastStore.getState().visible.map((t) => ({ level: t.level, message: t.message }));

describe('log out with the LogoutResult (FR-ACCT-004 UI, Ruling 101)', () => {
  it('a logout that Kite confirmed shows no note', async () => {
    const backend = createMockBackend(demoSeed());
    await expect(logOut(backend)).resolves.toBe(true);
    expect(backend.callsTo('session.logout')).toHaveLength(1);
    expect(toasts()).toEqual([]);
  });
  it('when Kite could not invalidate the token, a gentle note shows Go\'s message', async () => {
    const backend = createMockBackend(demoSeed());
    backend.session.logout = () => Promise.resolve({ remoteInvalidated: false, message: NOT_INVALIDATED });
    await expect(logOut(backend)).resolves.toBe(true);
    expect(toasts()).toEqual([{ level: 'info', message: NOT_INVALIDATED }]);
  });
  it('a remote failure without a message still gets a note', async () => {
    const backend = createMockBackend(demoSeed());
    backend.session.logout = () => Promise.resolve({ remoteInvalidated: false, message: '' });
    await logOut(backend);
    expect(toasts()).toEqual([{ level: 'info', message: 'Logged out. Kite could not be reached; the old Kite session ends by itself at 06:00 IST.' }]);
  });
  it('a refused logout shows the reason as an error and reports failure', async () => {
    const backend = createMockBackend(demoSeed());
    backend.failNext('session.logout', 'The secret store is not available', 'secret_store');
    await expect(logOut(backend)).resolves.toBe(false);
    expect(toasts()).toEqual([{ level: 'error', message: 'The secret store is not available' }]);
  });
});
