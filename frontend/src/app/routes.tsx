import { matchRoutes, type RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { HoldingsView } from '../views/holdings/HoldingsView';
import { NotFoundView } from '../views/NotFoundView';
import { LoginView } from '../views/onboarding/LoginView';
import { RiskDisclosureView } from '../views/onboarding/RiskDisclosureView';
import { SetupView } from '../views/onboarding/SetupView';
import { RestoreLastRoute } from './RestoreLastRoute';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5: `chart-full`) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
  { path: 'holdings', element: <HoldingsView /> },
];

/** Whether this build has a shell view at `path` (the not-found catch-all excluded) — Ruling 94 M4. */
export function isShellRoute(path: string): boolean {
  return (matchRoutes(shellChildren, path)?.length ?? 0) > 0;
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
