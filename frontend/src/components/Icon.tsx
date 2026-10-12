import type { ReactNode } from 'react';

const PATHS = {
  home: <path d="M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" />,
  scalper: <path d="M13 2L4 14h7l-1 8 9-12h-7z" />,
  strategies: (<><rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="3" y="14" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /></>),
  builder: <path d="M4 20h4l10-10-4-4L4 16zM14 6l4 4" />,
  live: (<><circle cx="12" cy="12" r="3" /><path d="M5 5a10 10 0 0 0 0 14M19 5a10 10 0 0 1 0 14" /></>),
  scanners: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" /></>),
  newScanner: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5M11 8v6M8 11h6" /></>),
  alerts: <path d="M6 16v-5a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0" />,
  orders: (<><rect x="5" y="3" width="14" height="18" rx="1" /><path d="M8 8h8M8 12h8M8 16h5" /></>),
  positions: <path d="M3 17l6-6 4 4 8-8M15 7h6v6" />,
  holdings: (<><rect x="3" y="7" width="18" height="13" rx="1" /><path d="M8 7V4h8v3" /></>),
  user: (<><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>),
  settings: (<><circle cx="12" cy="12" r="3" /><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9L7 7M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1" /></>),
  help: (<><circle cx="12" cy="12" r="9" /><path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.7M12 17h.01" /></>),
  info: (<><circle cx="12" cy="12" r="9" /><path d="M12 11v6M12 7h.01" /></>),
  back: <path d="M19 12H5M11 6l-6 6 6 6" />,
  refresh: <path d="M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5" />,
  search: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" /></>),
  close: <path d="M6 6l12 12M18 6L6 18" />,
  chevronLeft: <path d="M15 6l-6 6 6 6" />,
  chevronRight: <path d="M9 6l6 6-6 6" />,
  logout: <path d="M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10" />,
  copy: (<><rect x="8" y="8" width="12" height="12" rx="1" /><path d="M4 16V4h12" /></>),
  check: <path d="M5 12l5 5 9-10" />,
  warning: <path d="M12 3l10 18H2zM12 10v5M12 18h.01" />,
  error: (<><circle cx="12" cy="12" r="9" /><path d="M9 9l6 6M15 9l-6 6" /></>),
  plus: <path d="M12 5v14M5 12h14" />,
  trash: <path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" />,
  more: (<><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></>),
} satisfies Record<string, ReactNode>;

export type IconName = keyof typeof PATHS;

export interface IconProps { name: IconName; size?: number; className?: string }

export function Icon({ name, size = 18, className }: IconProps) {
  return (
    <svg
      className={className ? `icon ${className}` : 'icon'}
      width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"
    >
      {PATHS[name]}
    </svg>
  );
}
