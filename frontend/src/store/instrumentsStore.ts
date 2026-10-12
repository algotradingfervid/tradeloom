import { useEffect, useState } from 'react';
import { create } from 'zustand';
import type { Instrument } from '../instruments/types';
import { instrumentByIdentifier, instrumentsByTokens } from '../api/instruments';
import { instrumentId, isInScope } from '../instruments/naming';
import { registerReset } from './reset';
import { useSessionStore } from './session';

interface InstrumentsState {
  byToken: Record<number, Instrument>;
  byId: Record<string, number>;
  put(list: readonly Instrument[]): void;
}

export const useInstruments = create<InstrumentsState>()((set) => ({
  byToken: {},
  byId: {},
  put: (list) => set((s) => {
    const byToken = { ...s.byToken };
    const byId = { ...s.byId };
    for (const i of list) {
      if (!isInScope(i)) continue; // NFR-15: never cache an option, MCX or currency row
      byToken[i.token] = i;
      byId[instrumentId(i)] = i.token;
    }
    return { byToken, byId };
  }),
}));

const inflight = new Map<number, Promise<void>>();
const idInflight = new Map<string, Promise<Instrument>>();

// Late-answer guard (Rulings 96, 98, 102). A reset (logout, or the bridge's user switch) bumps the epoch; a call also
// drops its answer when the last known user changed to a different one while it was out. A session expiry of the same
// user cancels nothing. Go serves the instrument master to every session (it is not gated on login), but it
// re-imports the master after each login (service Deps.Refresher, "Trigger after login"), so the cache lives for one
// sign-in: the reset runs on logout (default onLogout: true).
let epoch = 0;
let lastUserId = useSessionStore.getState().session?.userId ?? '';
useSessionStore.subscribe((s) => {
  const id = s.session?.userId;
  if (id) lastUserId = id;
});

interface Guard { epoch: number; userId: string }
const guard = (): Guard => ({ epoch, userId: lastUserId });
const stillCurrent = (g: Guard): boolean =>
  g.epoch === epoch && (g.userId === '' || lastUserId === '' || g.userId === lastUserId);

registerReset(() => {
  epoch += 1;
  lastUserId = '';
  inflight.clear();
  idInflight.clear();
  useInstruments.setState({ byToken: {}, byId: {} });
});

/**
 * Loads the tokens not cached yet with one Instruments.ByTokens call; tokens already being loaded share that call.
 * Unknown tokens are omitted by Go and stay absent. An answer that arrives after a reset or a user switch is dropped
 * and the call resolves quietly.
 */
export async function ensureInstruments(tokens: readonly number[]): Promise<void> {
  const { byToken } = useInstruments.getState();
  const waits = tokens.map((t) => inflight.get(t)).filter((p): p is Promise<void> => p !== undefined);
  const missing = [...new Set(tokens)].filter((t) => !byToken[t] && !inflight.has(t));
  if (missing.length > 0) {
    const g = guard();
    const p: Promise<void> = instrumentsByTokens(missing)
      .then(
        (list) => { if (stillCurrent(g)) useInstruments.getState().put(list); },
        (e: unknown) => { if (stillCurrent(g)) throw e; },
      )
      .finally(() => missing.forEach((t) => { if (inflight.get(t) === p) inflight.delete(t); }));
    missing.forEach((t) => inflight.set(t, p));
    waits.push(p);
  }
  await Promise.all(waits);
}

/**
 * EXCHANGE:TRADINGSYMBOL → instrument, cached by identifier; concurrent lookups of one identifier share a call.
 * Rejects with Go's `not_found` (or `out_of_scope` from the typed helper). An answer that arrives after a reset is
 * returned to its caller but not cached.
 */
export async function resolveIdentifier(id: string): Promise<Instrument> {
  const s = useInstruments.getState();
  const token = s.byId[id];
  if (token !== undefined && s.byToken[token]) return s.byToken[token];
  const pending = idInflight.get(id);
  if (pending) return pending;
  const g = guard();
  const p: Promise<Instrument> = instrumentByIdentifier(id)
    .then((inst) => {
      if (stillCurrent(g)) useInstruments.getState().put([inst]);
      return inst;
    })
    .finally(() => { if (idInflight.get(id) === p) idInflight.delete(id); });
  idInflight.set(id, p);
  return p;
}

export function useInstrument(token: number | undefined): Instrument | undefined {
  const inst = useInstruments((s) => (token === undefined ? undefined : s.byToken[token]));
  useEffect(() => {
    if (token !== undefined && !inst) void ensureInstruments([token]).catch(() => undefined);
  }, [token, inst]);
  return inst;
}

export function useResolvedInstrument(id: string | undefined): { instrument?: Instrument; error?: string } {
  const [state, setState] = useState<{ id?: string; instrument?: Instrument; error?: string }>({});
  useEffect(() => {
    if (!id) return;
    let alive = true;
    resolveIdentifier(id)
      .then((instrument) => { if (alive) setState({ id, instrument }); })
      .catch((e: unknown) => { if (alive) setState({ id, error: e instanceof Error ? e.message : String(e) }); });
    return () => { alive = false; };
  }, [id]);
  return state.id === id ? { instrument: state.instrument, error: state.error } : {};
}
