import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <AppShell
      topBar={<TopBar ticker={null} />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => setCollapsed((c) => !c)} />}
    >
      <Outlet />
    </AppShell>
  );
}
