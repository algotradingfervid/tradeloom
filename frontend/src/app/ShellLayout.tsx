import { Outlet } from 'react-router-dom';
import { showOnDashboard } from '../store/chartContextsStore';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { IndexTickerStrip } from './IndexTickerStrip';
import { ProfileMenu } from './ProfileMenu';
import { QuickLinks } from './QuickLinks';
import { ShellBanners } from './ShellBanners';
import { ShellServices } from './ShellServices';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <AppShell
      topBar={<TopBar ticker={<IndexTickerStrip onShow={showOnDashboard} />} status={<StatusIndicator />} links={<QuickLinks />} profile={<ProfileMenu />} />}
      banner={<ShellBanners />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <ShellServices />
      <Outlet />
    </AppShell>
  );
}
