import { Outlet } from 'react-router-dom';
import { GlobalShortcuts } from '../shortcuts/GlobalShortcuts';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { BootNotice } from './BootNotice';
import { visibleNavEntries } from './features';
import { useRememberLastRoute } from './lastRoute';
import { Sidebar } from './Sidebar';
import { StatusIndicator } from './StatusIndicator';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  useRememberLastRoute();
  return (
    <AppShell
      topBar={<TopBar ticker={null} status={<StatusIndicator />} />}
      banner={<BootNotice />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <GlobalShortcuts />
      <Outlet />
    </AppShell>
  );
}
