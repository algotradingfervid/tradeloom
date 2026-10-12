import { chartHeaderCopy as copy } from '../../i18n/copy';
import { TIMEFRAMES, type TimeframeId } from '../timeframes';

/** Native select: arrows/Enter work out of the box (FR-CHART-004 keyboard). */
export function TimeframeSelect({ value, onChange }: { value: TimeframeId; onChange(tf: TimeframeId): void }) {
  return (
    <select className="chart-header__tf" aria-label={copy.timeframe} title={copy.timeframe} value={value}
      onChange={(e) => onChange(e.target.value as TimeframeId)}>
      {TIMEFRAMES.map((t) => <option key={t.id} value={t.id}>{t.label}</option>)}
    </select>
  );
}
