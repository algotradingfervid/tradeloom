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
  /**
   * Bumped when a load is dropped (reset, user switch) or the unknown-token set is cleared, so useInstrument /
   * useResolvedInstrument ask again instead of staying empty (Ruling 107 I1).
   */
  generation: number;
  put(list: readonly Instrument[]): void;
}

export const useInstruments = create<InstrumentsState>()((set) => ({
  byToken: {},
  byId: {},
  generation: 0,
  put: (list) => set((s) => {
    const fresh = list.filter(isInScope); // NFR-15: never cache an option, MCX or currency row
    if (fresh.length === 0) return s; // nothing added: no notification (Ruling 107 I2)
    const byToken = { ...s.byToken };
    const byId = { ...s.byId };
    for (const i of fresh) {
      byToken[i.token] = i;
      byId[instrumentId(i)] = i.token;
    }
    return { byToken, byId };
  }),
}));

const inflight = new Map<number, Promise<void>>();
const idInflight = new Map<string, Promise<Instrument>>();
/**
 * Tokens Go omitted from ByTokens (not in the instrument master). Not asked for again until the master can have
 * changed: Go re-imports it once per new login session (service.triggerInstrumentRefresh, data.go), which includes
 * expired(A) → ready(A) where nothing resets, so the set is cleared on every transition to ready and on reset.
 */
const unknown = new Set<number>();
const bumpGeneration = () => useInstruments.setState((s) => ({ generation: s.generation + 1 }));

// Late-answer guard (Rulings 96, 98, 102). A reset (logout, or the bridge's user switch) bumps the epoch; a call also
// drops its answer when the last known user changed to a different one while it was out. A session expiry of the same
// user cancels nothing. Go serves the instrument master to every session (it is not gated on login), but it
// re-imports the master once per new login session (data.go triggerInstrumentRefresh): the reset runs on logout (default
// onLogout: true), and since the daily re-import usually follows expired(A) → ready(A), which never resets, the
// unknown-token set is also cleared on every transition to ready (Ruling 107).
let epoch = 0;
let lastUserId = useSessionStore.getState().session?.userId ?? '';
useSessionStore.subscribe((s, prev) => {
  const id = s.session?.userId;
  if (id) lastUserId = id;
  if (s.session?.state === 'ready' && prev.session?.state !== 'ready' && unknown.size > 0) {
    unknown.clear();
    bumpGeneration(); // hooks showing an unknown token try again against the new import
  }
});

interface Guard { epoch: number; userId: string }
const guard = (): Guard => ({ epoch, userId: lastUserId });
const stillCurrent = (g: Guard): boolean =>
  g.epoch === epoch && (g.userId === '' || lastUserId === '' || g.userId === lastUserId);
/** A dropped answer: a reset already bumped the generation; a user switch did not, so bump it here. */
const dropped = (g: Guard): void => {
  if (g.epoch === epoch) bumpGeneration();
};

registerReset(() => {
  epoch += 1;
  lastUserId = '';
  inflight.clear();
  idInflight.clear();
  unknown.clear();
  useInstruments.setState((s) => ({ byToken: {}, byId: {}, generation: s.generation + 1 }));
});

/**
 * Loads the tokens not cached yet with one Instruments.ByTokens call; tokens already being loaded share that call.
 * Unknown tokens are omitted by Go and stay absent. An answer that arrives after a reset or a user switch is dropped
 * and the call resolves quietly.
 */
export async function ensureInstruments(tokens: readonly number[]): Promise<void> {
  const { byToken } = useInstruments.getState();
  const waits = tokens.map((t) => inflight.get(t)).filter((p): p is Promise<void> => p !== undefined);
  const missing = [...new Set(tokens)].filter((t) => !byToken[t] && !inflight.has(t) && !unknown.has(t));
  if (missing.length > 0) {
    const g = guard();
    // Settle before any state change, so a hook re-run by the generation bump starts a fresh load.
    const settle = () => missing.forEach((t) => { if (inflight.get(t) === p) inflight.delete(t); });
    const p: Promise<void> = instrumentsByTokens(missing).then(
      (list) => {
        settle();
        if (!stillCurrent(g)) return dropped(g);
        const found = new Set(list.map((i) => i.token));
        missing.forEach((t) => { if (!found.has(t)) unknown.add(t); });
        useInstruments.getState().put(list);
      },
      (e: unknown) => {
        settle();
        if (!stillCurrent(g)) return dropped(g);
        throw e;
      },
    );
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
export async function resolveIdentifier(raw: string): Promise<Instrument> {
  const id = raw.trim(); // Go: Instruments.ByID(strings.TrimSpace(id))
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
  const generation = useInstruments((s) => s.generation);
  useEffect(() => {
    if (token !== undefined && !inst) void ensureInstruments([token]).catch(() => undefined);
  }, [token, inst, generation]);
  return inst;
}

/**
 * A failed identifier lookup is asked again while the hook is mounted (Ruling 113 I2): on first launch the master may
 * not be imported yet and Go sends no event when it is, and identifier misses never enter the negative cache.
 */
export const RESOLVE_RETRY_MS: readonly number[] = [5_000, 15_000, 60_000];

export function useResolvedInstrument(id: string | undefined): { instrument?: Instrument; error?: string } {
  const generation = useInstruments((s) => s.generation);
  const [state, setState] = useState<{ id?: string; generation?: number; instrument?: Instrument; error?: string }>({});
  // Retry rounds for one (id, generation): `n` re-runs the lookup, `failures` walks the backoff. A new id or
  // generation (reset, user switch) starts over, and its effect cleanup clears any pending timer.
  const key = `${id ?? ''}\u0000${generation}`;
  const [retry, setRetry] = useState({ key, n: 0, failures: 0 });
  const failures = retry.key === key ? retry.failures : 0;
  useEffect(() => {
    if (!id) return;
    let alive = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    resolveIdentifier(id)
      .then((instrument) => { if (alive) setState({ id, generation, instrument }); })
      .catch((e: unknown) => {
        if (!alive) return;
        setState({ id, generation, error: e instanceof Error ? e.message : String(e) });
        const delay = RESOLVE_RETRY_MS[failures];
        if (delay !== undefined) {
          timer = setTimeout(() => setRetry((r) => ({ key, n: r.n + 1, failures: failures + 1 })), delay);
        }
      });
    return () => { alive = false; if (timer !== undefined) clearTimeout(timer); };
    // retry.n re-runs the lookup; failures and key change only together with retry.n, id or generation.
  }, [id, generation, retry.n, failures, key]);
  // An answer from before a reset or user switch (older generation) is never shown (Ruling 107 m4).
  const current = state.id === id && state.generation === generation;
  const errored = current && state.error !== undefined;
  // Every transition to ready may follow a new master import: an unresolved identifier asks again with a fresh backoff.
  useEffect(() => {
    if (!errored) return;
    return useSessionStore.subscribe((s, prev) => {
      if (s.session?.state === 'ready' && prev.session?.state !== 'ready') setRetry((r) => ({ key, n: r.n + 1, failures: 0 }));
    });
  }, [errored, key]);
  return current ? { instrument: state.instrument, error: state.error } : {};
}
