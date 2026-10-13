import { HomeLayout } from '../../app/HomeLayout';
import { homeCopy } from '../../i18n/copy';
import { HoldingsSummaryCard } from './HoldingsSummaryCard';
import { HomeChartArea, HomeWatchlistArea } from './slots';

/** FR-HOME-001. Later phases add the Live Strategy Overview card (P4), analytics (P1) and right-column widgets (P2/P3). */
export function HomeView() {
  return (
    <>
      <h1 className="visually-hidden">{homeCopy.title}</h1>
      <HomeLayout topRow={<HoldingsSummaryCard />} main={<HomeChartArea />} right={<HomeWatchlistArea />} />
    </>
  );
}
