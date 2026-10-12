import type { ReactNode } from 'react';
import './shell.css';

export interface AppShellProps { topBar: ReactNode; sidebar: ReactNode; banner?: ReactNode; children: ReactNode }

export function AppShell({ topBar, sidebar, banner, children }: AppShellProps) {
  return (
    <div className="shell">
      <header className="shell__top">{topBar}</header>
      <div className="shell__side">{sidebar}</div>
      <main className="shell__content" id="main-content">
        {banner}
        {children}
      </main>
    </div>
  );
}
