import { Outlet } from 'react-router-dom';
import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { BootNotice } from './BootNotice';
import { CalendarStaleCheck } from './CalendarStaleCheck';
import { CrashNotice } from './CrashNotice';
import { visibleNavEntries } from './features';
import { IndexTickerStrip } from './IndexTickerStrip';
import { ProfileMenu } from './ProfileMenu';
import { QuickLinks } from './QuickLinks';
import { useRememberLastRoute } from './lastRoute';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  useRememberLastRoute();
  return (
    <AppShell
      topBar={<TopBar ticker={<IndexTickerStrip />} status={<StatusIndicator />} links={<QuickLinks />} profile={<ProfileMenu />} />}
      // MERGE(P0-4 Task 9): move <CrashNotice /> (first) and <CalendarStaleCheck /> into ShellBanners; use banner--error.
      banner={<><CrashNotice /><BootNotice /><CalendarStaleCheck /></>}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <GlobalShortcuts />
      <Outlet />
    </AppShell>
  );
}
