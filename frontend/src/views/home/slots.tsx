import { DashboardChart } from '../../chart/ui/DashboardChart';
import { homeCopy } from '../../i18n/copy';
import './home.css';

/** The main chart (FR-CHART-001/002, P0-5 Task 10): the dashboard chart context, restored from chart.dashboard. */
export function HomeChartArea() {
  return (
    <section className="card home-slot home-slot--chart" aria-label={homeCopy.chartArea}>
      <DashboardChart />
    </section>
  );
}

/** P0-5 replaces this body with the watchlist panel (FR-WATCH-001). */
export function HomeWatchlistArea() {
  return <section className="card home-slot" aria-label={homeCopy.watchlistArea} />;
}
