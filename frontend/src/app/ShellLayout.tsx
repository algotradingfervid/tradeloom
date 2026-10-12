import { Outlet } from 'react-router-dom';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { BootNotice } from './BootNotice';
import { CalendarStaleCheck } from './CalendarStaleCheck';
import { visibleNavEntries } from './features';
import { ProfileMenu } from './ProfileMenu';
import { QuickLinks } from './QuickLinks';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <AppShell
      topBar={<TopBar ticker={null} status={<StatusIndicator />} links={<QuickLinks />} profile={<ProfileMenu />} />}
      // MERGE(P0-4 Task 9): move <CalendarStaleCheck /> into ShellBanners (Task 17 plan); it renders nothing.
      banner={<><BootNotice /><CalendarStaleCheck /></>}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <Outlet />
    </AppShell>
  );
}
