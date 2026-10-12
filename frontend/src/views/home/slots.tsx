import { homeCopy } from '../../i18n/copy';
import { WatchlistPanel } from '../watchlists/WatchlistPanel';
import './home.css';

/** P0-5 replaces this body with the main chart (FR-CHART-001/002). */
export function HomeChartArea() {
  return <section className="card home-slot" aria-label={homeCopy.chartArea} />;
}

/**
 * The watchlist panel (FR-WATCH-001). The slot keeps P0-4's card styling but is a plain div, so the page has one
 * "Watchlist" region (the panel's own section), not two nested ones.
 */
export function HomeWatchlistArea() {
  return (
    <div className="card home-slot">
      {/* MERGE(P0-5 Task 10): onChartItem={(item) => showOnDashboard(itemIdentifier(item))} — import showOnDashboard
          from '../../store/chartContextsStore' and itemIdentifier from '../../instruments/naming' once Task 10 lands. */}
      <WatchlistPanel onChartItem={() => undefined} />
    </div>
  );
}
