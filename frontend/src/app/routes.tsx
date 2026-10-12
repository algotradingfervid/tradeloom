import { matchRoutes, Navigate, type RouteObject } from 'react-router-dom';
import { HelpView } from '../help/HelpView';
import { HomeView } from '../views/home/HomeView';
import { HoldingsView } from '../views/holdings/HoldingsView';
import { LegalView } from '../views/legal/LegalView';
import { NotFoundView } from '../views/NotFoundView';
import { LoginView } from '../views/onboarding/LoginView';
import { RiskDisclosureView } from '../views/onboarding/RiskDisclosureView';
import { SetupView } from '../views/onboarding/SetupView';
import { RestoreLastRoute } from './RestoreLastRoute';
import { useRememberLastRoute } from './lastRoute';
import { OrderBookView } from '../views/orders/OrderBookView';
import { PositionsView } from '../views/positions/PositionsView';
import { ProfileView } from '../views/profile/ProfileView';
import { SettingsView, visibleSettingsGroups } from '../views/settings/SettingsView';
import { ChartFullView } from '../views/chart-full/ChartFullView';
import { DocsFrame } from './DocsFrame';
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
  { path: 'profile', element: <ProfileView /> },
];

/** Help and the legal pages: always reachable (gate ALWAYS_ALLOWED), framed by DocsFrame. */
export const docChildren: RouteObject[] = [
  { path: 'help/:article?', element: <HelpView /> },
  { path: 'legal/:page', element: <LegalView /> },
];

/** The full-view chart (P0-5 Task 11, FR-CHART-017) is remembered as the last view like a shell view. */
function ChartFullRoute() {
  useRememberLastRoute();
  return <ChartFullView />;
}

/** Root-level views outside the shell that the last-view restore may reopen. */
const restorableRootChildren: RouteObject[] = [{ path: 'chart-full', element: <ChartFullRoute /> }];

/** Whether this build has a shell view at `path` (the not-found catch-all excluded) — Ruling 94 M4. */
export function isShellRoute(path: string): boolean {
  // Help counts too (Ruling 117 I1): it opens in the shell once signed in. Legal pages are never remembered (lastRoute).
  const matches = matchRoutes([...shellChildren, ...docChildren, ...restorableRootChildren], path);
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
  // Ruling 108 M6 / 117 I1: DocsFrame shows the shell only when the session could open Home, so the risk screen's
  // links work before acknowledgement without the chrome (gate ALWAYS_ALLOWED).
  { element: <DocsFrame />, children: docChildren },
  // P0-5 Task 11: the full-view chart renders outside the shell (no top bar, sidebar or ticker), FR-CHART-017 AC3.
  ...restorableRootChildren,
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
