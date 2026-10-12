import { Navigate, type RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { HoldingsView } from '../views/holdings/HoldingsView';
import { NotFoundView } from '../views/NotFoundView';
import { OrderBookView } from '../views/orders/OrderBookView';
import { PositionsView } from '../views/positions/PositionsView';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
  { path: 'holdings', element: <HoldingsView /> },
  { path: 'positions', element: <PositionsView /> },
  { path: 'orders', element: <OrderBookView /> },
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
