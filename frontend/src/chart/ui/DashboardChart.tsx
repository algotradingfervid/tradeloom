import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { Skeleton } from '../../components/Skeleton';
import { chartViewCopy as copy } from '../../i18n/copy';
import { useShortcut } from '../../shortcuts/ShortcutProvider';
import { useSetting } from '../../store/settings';
import { DASHBOARD_CONTEXT, hydrateDashboard, useChartContexts } from '../../store/chartContextsStore';
import { openFullViewChart, takeFullViewReturn } from '../../views/chart-full/ChartFullView';
import { parseTimeframe } from '../timeframes';
import { ChartView } from './ChartView';

export function DashboardChart() {
  // Contract §E: Settings → Chart default timeframe, stored in Kite form ("60minute").
  const [defaultSetting] = useSetting<string>('chart.default_timeframe', '60minute');
  const defaultTf = parseTimeframe(defaultSetting);
  const ctx = useChartContexts((s) => s.contexts[DASHBOARD_CONTEXT]);
  const navigate = useNavigate();
  useEffect(() => { void hydrateDashboard(defaultTf); }, [defaultTf]);
  const openFull = () => {
    if (ctx) openFullViewChart(navigate, { identifier: ctx.identifier, timeframe: ctx.timeframe }, DASHBOARD_CONTEXT);
  };
  useShortcut('chart.fullView', openFull);
  // Back, Escape or F11 in the full view this chart opened: focus returns to its ⋯ "More chart actions" button.
  const ready = ctx !== undefined;
  useEffect(() => {
    if (!ready || !takeFullViewReturn(DASHBOARD_CONTEXT)) return;
    document.querySelector<HTMLElement>(`[data-context="${DASHBOARD_CONTEXT}"] .chart-header__menu [aria-haspopup="menu"]`)?.focus();
  }, [ready]);
  if (!ctx) return <div className="chart-view"><Skeleton rows={6} label={copy.loadingChart} /></div>;
  return (
    <ChartView contextId={DASHBOARD_CONTEXT} identifier={ctx.identifier} timeframe={ctx.timeframe}
      onSymbolChange={(id) => useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, id)}
      onTimeframeChange={(tf) => useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, tf)}
      onOpenFullView={openFull} />
  );
}
