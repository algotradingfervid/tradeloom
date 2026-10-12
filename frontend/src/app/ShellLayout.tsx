import { Outlet } from 'react-router-dom';
import { useSetting } from '../store/settings';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useSetting<boolean>('appearance.sidebar_collapsed', false);
  return (
    <AppShell
      topBar={<TopBar ticker={null} />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => void setCollapsed(!collapsed)} />}
    >
      <Outlet />
    </AppShell>
  );
}
