import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useLocation, useNavigate, type NavigateFunction } from 'react-router-dom';
import { ChartView } from '../../chart/ui/ChartView';
import { parseTimeframe, type TimeframeId } from '../../chart/timeframes';
import { Skeleton } from '../../components/Skeleton';
import { chartFullViewCopy as copy, chartViewCopy } from '../../i18n/copy';
import { DASHBOARD_CONTEXT, hydrateDashboard, useChartContexts } from '../../store/chartContextsStore';
import { useNavStore } from '../../store/nav';
import { registerReset } from '../../store/reset';
import { useSetting } from '../../store/settings';
import { useShortcut } from '../../shortcuts/ShortcutProvider';

export interface FullViewParams { identifier: string; timeframe: TimeframeId }
export const FULL_VIEW_CONTEXT = 'full';
export const FULL_VIEW_PATH = '/chart-full';

/**
 * Focus return. The opener's view unmounts while the full view is open, so its element cannot be refocused; the
 * opener names an origin instead (`openFullViewChart(…, origin)`) and takes the focus back when it mounts again
 * (`takeFullViewReturn(origin)`). `openedFrom`: origin of the full view on screen; `returnTo`: origin waiting for focus.
 */
let openedFrom: string | null = null;
let returnTo: string | null = null;

registerReset(() => { openedFrom = null; returnTo = null; }, { name: 'fullViewReturn', inspect: () => ({ openedFrom, returnTo }) });

/** Entry point for "Open in full view", F11 on Home and (P3) scanner result rows. */
export function openFullViewChart(navigate: NavigateFunction, params: FullViewParams, origin?: string): void {
  openedFrom = origin ?? null;
  returnTo = null;
  void navigate(FULL_VIEW_PATH, { state: params });
}

/** True once, when the user has just left a full view that `origin` opened: the origin then refocuses its opener. */
export function takeFullViewReturn(origin: string): boolean {
  if (returnTo !== origin) return false;
  returnTo = null;
  return true;
}

function paramsOf(state: unknown): FullViewParams | null {
  if (!state || typeof state !== 'object') return null;
  const s = state as Record<string, unknown>;
  return typeof s.identifier === 'string' ? { identifier: s.identifier, timeframe: parseTimeframe(s.timeframe) } : null;
}

/** Rendered from rootChildren, outside the shell: no top bar or sidebar, the chart fills the window (FR-CHART-017 AC3). */
export function ChartFullView() {
  const location = useLocation();
  const navigate = useNavigate();
  const mainRef = useRef<HTMLElement>(null);
  // Caller state (openFullViewChart) is read once. Without it (the restored last view, Ruling 119 I2) the full view
  // starts from the dashboard chart's symbol and timeframe, hydrated as DashboardChart does. A full view opened from
  // a P3 scanner row restores the same way: to the dashboard symbol, not the scanner's (no full-view UI-state key).
  const [caller] = useState(() => paramsOf(location.state));
  const [defaultSetting] = useSetting<string>('chart.default_timeframe', '60minute');
  const defaultTf = parseTimeframe(defaultSetting);
  const dashboard = useChartContexts((s) => s.contexts[DASHBOARD_CONTEXT]);
  useEffect(() => { if (!caller) void hydrateDashboard(defaultTf); }, [caller, defaultTf]);
  // Symbol and timeframe changes stay local to the full view: it never writes chart.dashboard.
  const [local, setLocal] = useState<Partial<FullViewParams>>({});
  const base = caller ?? dashboard;
  const identifier = local.identifier ?? base?.identifier;
  const timeframe = local.timeframe ?? base?.timeframe;
  // Back steps back to the origin when there is an entry behind this one, else opens Home (ListPage pattern).
  const back = () => {
    returnTo = openedFrom;
    openedFrom = null;
    if (useNavStore.getState().depth > 1) void navigate(-1);
    else void navigate('/home', { replace: true });
  };
  const backRef = useRef(back);
  useLayoutEffect(() => { backRef.current = back; });

  useShortcut('chart.fullView', back); // F11 again returns to the framed view it came from

  // Focus moves into the full view: the chart canvas, whose keys pan and zoom (once it exists, after hydration).
  const ready = identifier !== undefined && timeframe !== undefined;
  useEffect(() => { if (ready) mainRef.current?.querySelector<HTMLElement>('.chart-view__canvas')?.focus(); }, [ready]);

  // Escape leaves the full view, unless something on top handled it (a dialog, a menu) or a modal dialog is open.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape' || e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
      if (e.target instanceof Element && e.target.closest('[aria-modal="true"]')) return;
      e.preventDefault();
      backRef.current();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);

  return (
    <main ref={mainRef} className="chart-full" aria-label={copy.title}>
      <h1 className="visually-hidden">{copy.title}</h1>
      {ready ? (
        <ChartView contextId={FULL_VIEW_CONTEXT} identifier={identifier} timeframe={timeframe}
          onSymbolChange={(id) => setLocal((l) => ({ ...l, identifier: id }))}
          onTimeframeChange={(tf) => setLocal((l) => ({ ...l, timeframe: tf }))} onBack={back} />
      ) : (
        <div className="chart-view"><Skeleton rows={6} label={chartViewCopy.loadingChart} /></div>
      )}
    </main>
  );
}
