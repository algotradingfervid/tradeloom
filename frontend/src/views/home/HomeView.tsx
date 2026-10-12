import { HomeLayout } from '../../app/HomeLayout';
import { homeCopy } from '../../i18n/copy';

export function HomeView() {
  return (
    <>
      <h1 className="visually-hidden">{homeCopy.title}</h1>
      <HomeLayout
        main={<section className="card" aria-label={homeCopy.chartArea} />}
        right={<section className="card" aria-label={homeCopy.watchlistArea} />}
      />
    </>
  );
}
