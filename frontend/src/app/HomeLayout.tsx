import type { ReactNode } from 'react';
import { homeCopy } from '../i18n/copy';

export interface HomeLayoutProps { topRow?: ReactNode; main: ReactNode; below?: ReactNode; right: ReactNode }

export function HomeLayout({ topRow, main, below, right }: HomeLayoutProps) {
  return (
    <div className="home">
      <div className="home__left" data-testid="home-left">
        {topRow && <div className="home__top-row">{topRow}</div>}
        <div className="home__main">{main}</div>
        {below}
      </div>
      <aside className="home__right" data-testid="home-right" aria-label={homeCopy.rightColumn}>
        {right}
      </aside>
    </div>
  );
}
