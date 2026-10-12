import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useLocation, useNavigate, type NavigateFunction } from 'react-router-dom';
import { ChartView } from '../../chart/ui/ChartView';
import { parseTimeframe, SHIPPED_DEFAULT_TIMEFRAME, type TimeframeId } from '../../chart/timeframes';
import { chartFullViewCopy as copy } from '../../i18n/copy';
import { DEFAULT_DASHBOARD_SYMBOL } from '../../store/chartContextsStore';
import { registerReset } from '../../store/reset';
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

function paramsOf(state: unknown): Partial<FullViewParams> | null {
  if (!state || typeof state !== 'object') return null;
  const s = state as Record<string, unknown>;
  return { identifier: typeof s.identifier === 'string' ? s.identifier : undefined, timeframe: s.timeframe as TimeframeId | undefined };
}

/** Rendered from rootChildren, outside the shell: no top bar or sidebar, the chart fills the window (FR-CHART-017 AC3). */
export function ChartFullView() {
  const location = useLocation();
  const navigate = useNavigate();
  const mainRef = useRef<HTMLElement>(null);
  const initial = paramsOf(location.state);
  const [identifier, setIdentifier] = useState(initial?.identifier ?? DEFAULT_DASHBOARD_SYMBOL);
  const [timeframe, setTimeframe] = useState<TimeframeId>(parseTimeframe(initial?.timeframe ?? SHIPPED_DEFAULT_TIMEFRAME));
  // Opened by openFullViewChart, the entry below is the origin. Without caller state (the restored last view) there
  // is nothing to go back to, so Back opens Home.
  const fromCaller = initial !== null;
  const back = () => {
    returnTo = openedFrom;
    openedFrom = null;
    if (fromCaller) void navigate(-1);
    else void navigate('/home', { replace: true });
  };
  const backRef = useRef(back);
  useLayoutEffect(() => { backRef.current = back; });

  useShortcut('chart.fullView', back); // F11 again returns to the framed view it came from

  // Focus moves into the full view: the chart canvas, whose keys pan and zoom.
  useEffect(() => { mainRef.current?.querySelector<HTMLElement>('.chart-view__canvas')?.focus(); }, []);

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
      <ChartView contextId={FULL_VIEW_CONTEXT} identifier={identifier} timeframe={timeframe}
        onSymbolChange={setIdentifier} onTimeframeChange={setTimeframe} onBack={back} />
    </main>
  );
}
