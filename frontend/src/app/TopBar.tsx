import type { ReactNode } from 'react';
import { shellCopy } from '../i18n/copy';

export interface TopBarProps { ticker?: ReactNode; status?: ReactNode; links?: ReactNode; profile?: ReactNode }

export function TopBar({ ticker, status, links, profile }: TopBarProps) {
  return (
    <div className="topbar">
      <div className="topbar__ticker" data-testid="topbar-ticker">{ticker}</div>
      <div className="topbar__status">{status}</div>
      <div className="topbar__spacer" />
      <nav className="topbar__links" aria-label={shellCopy.quickLinks}>{links}</nav>
      <div className="topbar__profile">{profile}</div>
    </div>
  );
}
