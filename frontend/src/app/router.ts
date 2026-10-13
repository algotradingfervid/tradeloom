import { createMemoryRouter } from 'react-router-dom';
import { buildRoutes } from './routes';

export type AppRouter = ReturnType<typeof createMemoryRouter>;

export function createAppRouter(initialEntries: string[] = ['/']): AppRouter {
  return createMemoryRouter(buildRoutes(), { initialEntries });
}
