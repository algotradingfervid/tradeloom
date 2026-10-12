import { useId } from 'react';
import { Icon } from '../../components/Icon';
import { IconButton } from '../../components/IconButton';
import { MenuButton } from '../../components/MenuButton';
import { InstrumentLabel } from '../../components/instrument/InstrumentLabel';
import { PriceChange } from '../../components/instrument/PriceChange';
import { chartHeaderCopy as copy } from '../../i18n/copy';
import { tickRupees } from '../../instruments/naming';
import type { Instrument } from '../../instruments/types';
import { useSubscription } from '../../market/hooks';
import { precisionForTick } from '../format';
import type { TimeframeId } from '../timeframes';
import { TimeframeSelect } from './TimeframeSelect';
import './chart.css';

export interface ChartHeaderProps {
  contextId: string;
  instrument: Instrument | undefined;
  timeframe: TimeframeId;
  refreshing: boolean;
  refreshDisabled: boolean;
  onOpenSearch(): void;
  onRefresh(): void;
  onTimeframe(tf: TimeframeId): void;
  onOpenFullView?: () => void;
  onBack?: () => void;
}

/**
 * FR-CHART-002: symbol (opens search), live LTP and change, refresh, timeframe and the ⋯ menu. No order actions (G-0).
 * The header's token is subscribed as ticker consumer `chart` under `chart-header:<contextId>`; useSubscription moves
 * the set on token change and clears it on unmount.
 */
export function ChartHeader(p: ChartHeaderProps) {
  useSubscription(`chart-header:${p.contextId}`, 'chart', p.instrument ? [p.instrument.token] : []);
  const summaryId = useId();
  return (
    <div className="chart-header" role="toolbar" aria-label={copy.controls}>
      {p.onBack && <IconButton label={copy.back} icon="back" onClick={p.onBack} />}
      <button type="button" className="chart-header__symbol" title={copy.searchInstruments} aria-label={copy.changeSymbol}
        aria-describedby={summaryId} onClick={p.onOpenSearch}>
        <span aria-hidden="true" className="chart-header__search-icon"><Icon name="search" size={16} /></span>
        <span id={summaryId} className="chart-header__summary">
          {p.instrument ? (
            <>
              <InstrumentLabel instrument={p.instrument} />
              <PriceChange token={p.instrument.token} decimals={precisionForTick(tickRupees(p.instrument))} />
            </>
          ) : <span>—</span>}
        </span>
      </button>
      <IconButton label={copy.refresh} icon="refresh" className={p.refreshing ? 'is-spinning' : undefined}
        aria-busy={p.refreshing || undefined} disabled={p.refreshing || p.refreshDisabled} onClick={p.onRefresh} />
      <TimeframeSelect value={p.timeframe} onChange={p.onTimeframe} />
      {p.onOpenFullView && (
        <div className="chart-header__menu">
          <MenuButton label={copy.more} trigger={<span className="icon-btn" aria-hidden="true"><Icon name="more" size={18} /></span>}
            items={[{ kind: 'item', id: 'full-view', label: copy.openFullView, onSelect: () => p.onOpenFullView?.() }]} />
        </div>
      )}
    </div>
  );
}
