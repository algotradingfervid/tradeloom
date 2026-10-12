import { render } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { demoSeed } from '../api/mock/fixtures';
import { createMockBackend, type MockBackend } from '../api/mock/index';
import { App } from '../app/App';
import { createAppRouter } from '../app/router';

export interface RenderAppOptions { route?: string; backend?: MockBackend }

export function renderApp({ route = '/home', backend = createMockBackend(demoSeed()) }: RenderAppOptions = {}) {
  const router = createAppRouter([route]);
  const user = userEvent.setup();
  const view = render(<App backend={backend} router={router} />);
  return { ...view, backend, router, user };
}
