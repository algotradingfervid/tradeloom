import { Navigate, type RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { HoldingsView } from '../views/holdings/HoldingsView';
import { NotFoundView } from '../views/NotFoundView';
import { CalendarGroup } from '../views/settings/CalendarGroup';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
  // MERGE(P0-4 Task 14): delete this route and its import; add `calendar: CalendarGroup` to GROUP_COMPONENTS in
  // SettingsView.tsx instead (Task 14's `settings/:group` route then serves /settings/calendar).
  { path: 'settings/calendar', element: <div className="settings__panel"><CalendarGroup /></div> },
  { path: 'holdings', element: <HoldingsView /> },
];

/** Children of the root layout that render outside the shell (onboarding) plus the shell itself. */
export const rootChildren: RouteObject[] = [
  { index: true, element: <Navigate to="/home" replace /> },
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
