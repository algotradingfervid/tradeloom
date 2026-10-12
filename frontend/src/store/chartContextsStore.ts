import { create } from 'zustand';
import { readUiState, writeUiState } from '../api/uiState';
import { parseTimeframe, type TimeframeId } from '../chart/timeframes';
import { registerReset } from './reset';

export interface ChartContextState { identifier: string; timeframe: TimeframeId }
export const DASHBOARD_CONTEXT = 'dashboard';
/** Contract §E: UI-state key for the dashboard chart, JSON {identifier, timeframe}. */
export const DASHBOARD_UI_STATE_KEY = 'chart.dashboard';
export const DEFAULT_DASHBOARD_SYMBOL = 'NSE:NIFTY 50';
const PERSISTED: Readonly<Record<string, string>> = { [DASHBOARD_CONTEXT]: DASHBOARD_UI_STATE_KEY }; // Scalper slots join in P6

interface ChartContextsStore {
  contexts: Record<string, ChartContextState>;
  init(ctx: string, initial: ChartContextState): void;
  setSymbol(ctx: string, identifier: string): void;
  setTimeframe(ctx: string, timeframe: TimeframeId): void;
}

export const useChartContexts = create<ChartContextsStore>()((set, get) => {
  const update = (ctx: string, patch: Partial<ChartContextState>) => {
    const cur = get().contexts[ctx];
    if (!cur) return;
    const next = { ...cur, ...patch };
    set((s) => ({ contexts: { ...s.contexts, [ctx]: next } }));
    const key = PERSISTED[ctx];
    if (key) void writeUiState(key, next).catch(() => undefined);
  };
  return {
    contexts: {},
    init: (ctx, initial) => {
      if (get().contexts[ctx]) return;
      set((s) => ({ contexts: { ...s.contexts, [ctx]: initial } }));
    },
    setSymbol: (ctx, identifier) => update(ctx, { identifier }),
    setTimeframe: (ctx, timeframe) => update(ctx, { timeframe }),
  };
});

let hydrating: Promise<void> | null = null;
let pendingSymbol: string | null = null;
/** Bumped by every reset: a hydrate that started earlier must not repopulate the store. */
let epoch = 0;

export function hydrateDashboard(defaultTimeframe: TimeframeId): Promise<void> {
  if (useChartContexts.getState().contexts[DASHBOARD_CONTEXT]) return Promise.resolve();
  const started = epoch;
  hydrating ??= readUiState<Partial<ChartContextState> | null>(DASHBOARD_UI_STATE_KEY, null)
    .catch(() => null)
    .then((saved) => {
      if (started !== epoch) return;
      const store = useChartContexts.getState();
      store.init(DASHBOARD_CONTEXT, saved && typeof saved.identifier === 'string'
        ? { identifier: saved.identifier, timeframe: parseTimeframe(saved.timeframe) }
        : { identifier: DEFAULT_DASHBOARD_SYMBOL, timeframe: defaultTimeframe });
      if (pendingSymbol) { store.setSymbol(DASHBOARD_CONTEXT, pendingSymbol); pendingSymbol = null; }
    })
    .finally(() => { if (started === epoch) hydrating = null; });
  return hydrating;
}

/** Ticker pills / watchlist rows: load an instrument into the dashboard chart (even before Home has mounted). */
export function showOnDashboard(identifier: string): void {
  const store = useChartContexts.getState();
  if (store.contexts[DASHBOARD_CONTEXT]) store.setSymbol(DASHBOARD_CONTEXT, identifier);
  else pendingSymbol = identifier;
}

export function __resetChartContexts(): void {
  epoch += 1;
  useChartContexts.setState({ contexts: {} });
  hydrating = null;
  pendingSymbol = null;
}

/**
 * Chart contexts are install-wide, not account data: Go keeps chart.dashboard in window_state (one row per key, no
 * user column; Settings.GetUiState is not gated on the session), so sign-out keeps them (onLogout: false). Only
 * resetAllStores (tests) clears them.
 */
registerReset(__resetChartContexts, {
  onLogout: false,
  name: 'chartContexts',
  inspect: () => ({ ...useChartContexts.getState(), epoch, pendingSymbol, hydrating: hydrating !== null }),
});
