import { useEffect, useReducer, useRef, useState, type KeyboardEvent } from 'react';
import { Button } from '../../components/Button';
import { Skeleton } from '../../components/Skeleton';
import { closeLive, fetchHistory, onBackfillProgress, onCandle, openLive, refreshRange } from '../../api/chart';
import { useTheme } from '../../app/theme';
import { chartViewCopy as copy } from '../../i18n/copy';
import { useSessionStore } from '../../store/session';
import { useResolvedInstrument } from '../../store/instrumentsStore';
import { instrumentId, tickRupees } from '../../instruments/naming';
import type { ChartAdapter } from '../ChartAdapter';
import { LightweightChartsAdapter } from '../LightweightChartsAdapter';
import { ChartDataController, INITIAL_CHART_STATE, NO_DATA_PLAN_MESSAGE, type ChartDataApi, type ChartLoadState } from '../ChartDataController';
import { precisionForTick } from '../format';
import { readChartTheme } from '../theme';
import type { TimeframeId } from '../timeframes';
import type { LegendData } from '../types';
import { ChartHeader } from './ChartHeader';
import { ChartLegend } from './ChartLegend';
import { ScaleToggles } from './ScaleToggles';
import { SymbolSearchDialog } from './SymbolSearchDialog';
import './chart.css';

export interface ChartViewProps {
  contextId: string;
  identifier: string;
  timeframe: TimeframeId;
  onSymbolChange(identifier: string): void;
  onTimeframeChange(tf: TimeframeId): void;
  onOpenFullView?: () => void;
  onBack?: () => void;
  maxStudies?: number;
  adapterFactory?: (el: HTMLElement) => ChartAdapter;
  dataApi?: ChartDataApi;
}

const defaultDataApi: ChartDataApi = { fetchHistory, refreshRange, openLive, closeLive, onCandle, onBackfillProgress };

export function ChartView(props: ChartViewProps) {
  const { contextId, identifier, timeframe } = props;
  const hostRef = useRef<HTMLDivElement>(null);
  const [adapter, setAdapter] = useState<ChartAdapter | null>(null);
  const [controller, setController] = useState<ChartDataController | null>(null);
  const [state, setState] = useState<ChartLoadState>(INITIAL_CHART_STATE);
  const [hover, setHover] = useState<LegendData | null>(null);
  const [searchOpen, setSearchOpen] = useState(false);
  const [, bump] = useReducer((x: number) => x + 1, 0);
  const { instrument, error: resolveError } = useResolvedInstrument(identifier);
  // "No data plan" comes from the session (contract §D), never from feed.state.
  const noPlan = useSessionStore((s) => s.session?.dataPlan === 'none');
  // Candles load for a signed-in user (an expired session still serves cached candles). After a sign-out or a user
  // switch the controller sits idle until then (ChartLoadState.resetEpoch, Ruling 109 I3).
  const signedIn = useSessionStore((s) => s.session?.state === 'ready' || s.session?.state === 'expired');
  const userId = useSessionStore((s) => s.session?.userId ?? '');
  const { tokens } = useTheme();

  useEffect(() => {
    const el = hostRef.current!;
    const a = props.adapterFactory ? props.adapterFactory(el) : new LightweightChartsAdapter(el, { maxStudies: props.maxStudies });
    const c = new ChartDataController(a, props.dataApi ?? defaultDataApi, setState, bump);
    // The adapter reports the last bar when the pointer is on it or has left the chart. Storing that report would
    // freeze the legend on it while live candles replace or follow it (Task 4 review I1), so it is never stored:
    // the legend then reads the live last bar.
    const offCross = a.onCrosshairMove((d) => setHover(d && d.bar.time === a.bars().at(-1)?.time ? null : d));
    setAdapter(a);
    setController(c);
    return () => { offCross(); c.dispose(); a.destroy(); };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- one adapter per mounted view
  }, []);

  useEffect(() => { adapter?.setTheme(readChartTheme(tokens)); }, [adapter, tokens]);

  useEffect(() => {
    if (!controller || !instrument || noPlan || !signedIn) return;
    setHover(null);
    void controller.load(instrument, timeframe);
  }, [controller, instrument, timeframe, noPlan, signedIn]);

  // Reload after a reset (sign-out, or the bridge's user switch): the controller went idle and bumped resetEpoch.
  // Declared after the load effect, so a load that just started (status 'loading') is not repeated.
  useEffect(() => {
    if (!controller || noPlan || !signedIn) return;
    if (controller.current.status === 'idle') void controller.retry();
  }, [controller, state.resetEpoch, signedIn, userId, noPlan]);

  // A reset cleared the chart: a hover from before it (previous user's candles) must not show over the reload.
  useEffect(() => { setHover(null); }, [state.resetEpoch]);

  const bars = adapter?.bars() ?? [];
  const last = bars.length > 0 ? bars[bars.length - 1] : undefined;
  const lastLegend: LegendData | null = last ? { bar: last, previousClose: bars.length > 1 ? bars[bars.length - 2].close : null } : null;
  const legend = hover ?? lastLegend;
  const decimals = instrument ? precisionForTick(tickRupees(instrument)) : 2;

  // Read on every render: a drag on the price axis turns auto-scale off inside the chart, without a callback.
  const scale = adapter?.priceScaleState() ?? { log: false, auto: true };
  const toggleAuto = () => { if (!adapter) return; adapter.setPriceScale({ auto: true }); bump(); };
  const toggleLog = () => { if (!adapter) return; adapter.setPriceScale({ log: !adapter.priceScaleState().log }); bump(); };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!adapter || e.altKey || e.ctrlKey || e.metaKey) return;
    switch (e.key) {
      case 'ArrowLeft': adapter.panBars(-10); break;
      case 'ArrowRight': adapter.panBars(10); break;
      case '+': case '=': adapter.zoom(1.25); break;
      case '-': adapter.zoom(0.8); break;
      case 'Home': adapter.scrollToLatest(); break;
      case 'a': case 'A': toggleAuto(); break;
      case 'l': case 'L': toggleLog(); break;
      default: return;
    }
    e.preventDefault();
  };

  const status = noPlan ? 'no_data_plan' : state.status;
  // An identifier that does not resolve replaces the load overlays (they belong to the previous instrument).
  const overlays = !resolveError;
  const showsCandles = status === 'ready' || status === 'empty';
  return (
    <div className="chart-view" data-context={contextId} data-shortcut-scope="chart">
      <ChartHeader contextId={contextId} instrument={instrument} timeframe={timeframe}
        refreshing={state.refreshing} refreshDisabled={status !== 'ready'}
        onOpenSearch={() => setSearchOpen(true)} onRefresh={() => void controller?.refresh()}
        onTimeframe={props.onTimeframeChange} onOpenFullView={props.onOpenFullView} onBack={props.onBack} />
      <div className="chart-view__body">
        <div ref={hostRef} className="chart-view__canvas" tabIndex={0} role="region" aria-label={copy.canvas} onKeyDown={onKeyDown} />
        <ChartLegend data={status === 'ready' ? legend : null} hasVolume={!!instrument && !instrument.isIndex} decimals={decimals} />
        <ScaleToggles auto={scale.auto} log={scale.log} onAuto={toggleAuto} onLog={toggleLog} />
        <div className="chart-view__badges">
          {state.offline && status === 'ready' && <span className="chart-view__badge">{copy.offline}</span>}
          {/* Go refused OpenLive (subscription_budget, not_found, …): history shows, its reason says why live is off. */}
          {showsCandles && state.liveMessage && (
            <span className="chart-view__badge chart-view__badge--live" role="status" title={copy.liveOff}>{state.liveMessage}</span>
          )}
          {status === 'ready' && state.message && <span className="chart-view__badge" role="alert">{state.message}</span>}
        </div>
        {overlays && status === 'loading' && (
          <div className="chart-view__overlay" data-testid="chart-loading"><Skeleton rows={6} label={copy.loadingCandles} /></div>
        )}
        {overlays && status === 'no_data_plan' && <div className="chart-view__overlay" role="status">{NO_DATA_PLAN_MESSAGE}</div>}
        {overlays && status === 'empty' && <div className="chart-view__overlay" role="status">{state.message}</div>}
        {overlays && status === 'error' && (
          <div className="chart-view__overlay" role="alert">
            <span>{state.message}</span>
            <Button onClick={() => void controller?.retry()}>{copy.retry}</Button>
          </div>
        )}
        {resolveError && <div className="chart-view__overlay" role="alert">{resolveError}</div>}
      </div>
      <SymbolSearchDialog open={searchOpen} onClose={() => setSearchOpen(false)}
        onSelect={(i) => props.onSymbolChange(instrumentId(i))} />
    </div>
  );
}
