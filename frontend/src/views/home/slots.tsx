import { homeCopy } from '../../i18n/copy';
import './home.css';

/** P0-5 replaces this body with the main chart (FR-CHART-001/002). */
export function HomeChartArea() {
  return <section className="card home-slot" aria-label={homeCopy.chartArea} />;
}

/** P0-5 replaces this body with the watchlist panel (FR-WATCH-001). */
export function HomeWatchlistArea() {
  return <section className="card home-slot" aria-label={homeCopy.watchlistArea} />;
}
