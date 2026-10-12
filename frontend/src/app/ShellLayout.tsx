import { Outlet } from 'react-router-dom';
import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { ProfileMenu } from './ProfileMenu';
import { QuickLinks } from './QuickLinks';
import { ShellBanners } from './ShellBanners';
import { useRememberLastRoute } from './lastRoute';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  useRememberLastRoute();
  return (
    <AppShell
      topBar={<TopBar ticker={null} status={<StatusIndicator />} links={<QuickLinks />} profile={<ProfileMenu />} />}
      banner={<ShellBanners />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <GlobalShortcuts />
      <Outlet />
    </AppShell>
  );
}
