import { matchRoutes, Navigate, type RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { HoldingsView } from '../views/holdings/HoldingsView';
import { NotFoundView } from '../views/NotFoundView';
import { LoginView } from '../views/onboarding/LoginView';
import { RiskDisclosureView } from '../views/onboarding/RiskDisclosureView';
import { SetupView } from '../views/onboarding/SetupView';
import { RestoreLastRoute } from './RestoreLastRoute';
import { OrderBookView } from '../views/orders/OrderBookView';
import { PositionsView } from '../views/positions/PositionsView';
import { SettingsView, visibleSettingsGroups } from '../views/settings/SettingsView';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5: `chart-full`) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
  { path: 'holdings', element: <HoldingsView /> },
  { path: 'positions', element: <PositionsView /> },
  { path: 'orders', element: <OrderBookView /> },
  { path: 'settings', element: <Navigate to="/settings/kite" replace /> },
  { path: 'settings/:group', element: <SettingsView /> },
];

/** Whether this build has a shell view at `path` (the not-found catch-all excluded) — Ruling 94 M4. */
export function isShellRoute(path: string): boolean {
  const matches = matchRoutes(shellChildren, path);
  const leaf = matches?.[matches.length - 1];
  if (!leaf) return false;
  // Ruling 108 M7: a Settings group exists only if this build shows it (hidden groups would open "Page not found").
  if (leaf.route.path === 'settings/:group') return visibleSettingsGroups().some((g) => g.id === leaf.params.group);
  return true;
}

/** Root-level children rendered without the shell (onboarding); the index reopens the last view. */
export const rootChildren: RouteObject[] = [
  { index: true, element: <RestoreLastRoute exists={isShellRoute} /> },
  { path: 'setup', element: <SetupView /> },
  { path: 'login', element: <LoginView /> },
  { path: 'risk', element: <RiskDisclosureView /> },
];

export function buildRoutes(): RouteObject[] {
  return [
    {
      path: '/',
      element: <RootLayout />,
      children: [...rootChildren, { element: <ShellLayout />, children: [...shellChildren, { path: '*', element: <NotFoundView /> }] }],
    },
  ];
}
