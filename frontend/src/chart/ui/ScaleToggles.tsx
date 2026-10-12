import { chartViewCopy as copy } from '../../i18n/copy';

/** Text toggles, so plain buttons with aria-pressed rather than icon buttons (FR-CHART-011 AC2). */
export function ScaleToggles({ auto, log, onAuto, onLog }: { auto: boolean; log: boolean; onAuto(): void; onLog(): void }) {
  return (
    <div className="chart-scale-toggles" role="group" aria-label={copy.priceScale}>
      <button type="button" aria-pressed={auto} title={copy.autoScale} onClick={onAuto}>A</button>
      <button type="button" aria-pressed={log} title={copy.logScale} onClick={onLog}>L</button>
    </div>
  );
}
