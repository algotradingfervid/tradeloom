import { useEffect } from 'react';
import { Skeleton } from '../../components/Skeleton';
import { chartViewCopy as copy } from '../../i18n/copy';
import { useSetting } from '../../store/settings';
import { DASHBOARD_CONTEXT, hydrateDashboard, useChartContexts } from '../../store/chartContextsStore';
import { parseTimeframe } from '../timeframes';
import { ChartView } from './ChartView';

export function DashboardChart() {
  // Contract §E: Settings → Chart default timeframe, stored in Kite form ("60minute").
  const [defaultSetting] = useSetting<string>('chart.default_timeframe', '60minute');
  const defaultTf = parseTimeframe(defaultSetting);
  const ctx = useChartContexts((s) => s.contexts[DASHBOARD_CONTEXT]);
  useEffect(() => { void hydrateDashboard(defaultTf); }, [defaultTf]);
  if (!ctx) return <div className="chart-view"><Skeleton rows={6} label={copy.loadingChart} /></div>;
  return (
    <ChartView contextId={DASHBOARD_CONTEXT} identifier={ctx.identifier} timeframe={ctx.timeframe}
      onSymbolChange={(id) => useChartContexts.getState().setSymbol(DASHBOARD_CONTEXT, id)}
      onTimeframeChange={(tf) => useChartContexts.getState().setTimeframe(DASHBOARD_CONTEXT, tf)} />
  );
}
