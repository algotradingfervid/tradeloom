import { useRef, type KeyboardEvent } from 'react';
import { NavLink } from 'react-router-dom';
import { Icon } from '../components/Icon';
import { IconButton } from '../components/IconButton';
import { Tooltip } from '../components/Tooltip';
import { shellCopy } from '../i18n/copy';
import type { NavEntry } from './features';

export interface SidebarProps { entries: readonly NavEntry[]; collapsed: boolean; onToggleCollapsed: () => void }

export function Sidebar({ entries, collapsed, onToggleCollapsed }: SidebarProps) {
  const links = useRef<(HTMLAnchorElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const n = entries.length;
    links.current[(index + (e.key === 'ArrowDown' ? 1 : -1) + n) % n]?.focus();
  };

  return (
    <nav className={collapsed ? 'sidebar sidebar--collapsed' : 'sidebar'} aria-label={shellCopy.sidebarLabel}>
      <ul>
        {entries.map((entry, i) => {
          const link = (
            <NavLink
              ref={(el) => { links.current[i] = el; }}
              to={entry.path}
              className="sidebar__link"
              aria-label={collapsed ? entry.label : undefined}
              onKeyDown={(e) => onKeyDown(e, i)}
            >
              <Icon name={entry.icon} />
              <span className="sidebar__label">{entry.label}</span>
            </NavLink>
          );
          return <li key={entry.id}>{collapsed ? <Tooltip label={entry.label} placement="right">{link}</Tooltip> : link}</li>;
        })}
      </ul>
      <div className="sidebar__collapse">
        <IconButton
          label={collapsed ? shellCopy.expandSidebar : shellCopy.collapseSidebar}
          icon={collapsed ? 'chevronRight' : 'chevronLeft'}
          onClick={onToggleCollapsed}
          aria-expanded={!collapsed}
        />
      </div>
    </nav>
  );
}
