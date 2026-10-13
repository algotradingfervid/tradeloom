import { formatCompact, formatPercent, formatPrice, formatSigned } from '../../i18n/format';
import type { LegendData } from '../types';

export function ChartLegend({ data, hasVolume, decimals }: { data: LegendData | null; hasVolume: boolean; decimals: number }) {
  if (!data) return <div className="chart-legend" data-testid="chart-legend" />;
  const { bar, previousClose } = data;
  const up = bar.close >= bar.open;
  const change = previousClose !== null && previousClose > 0 ? bar.close - previousClose : null;
  const p = (v: number) => formatPrice(v, decimals);
  return (
    <div className={`chart-legend chart-legend--${up ? 'up' : 'down'}`} data-testid="chart-legend">
      <span>O <b>{p(bar.open)}</b></span>
      <span>H <b>{p(bar.high)}</b></span>
      <span>L <b>{p(bar.low)}</b></span>
      <span>C <b>{p(bar.close)}</b></span>
      {change !== null && previousClose !== null && (
        <span><b>{formatSigned(change, decimals)} ({formatPercent((change / previousClose) * 100)})</b></span>
      )}
      {hasVolume && <span className="chart-legend__volume">Vol <b>{formatCompact(bar.volume)}</b></span>}
    </div>
  );
}
