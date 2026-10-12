import { render, screen, within } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { HomeLayout } from './HomeLayout';
import { buildRoutes } from './routes';

describe('FR-SHELL-001 frame', () => {
  it('AC1 (P0 form): top bar, sidebar and content are visible; the sidebar holds the P0 entries', async () => {
    const router = createMemoryRouter(buildRoutes(), { initialEntries: ['/home'] });
    render(<RouterProvider router={router} />);
    expect(await screen.findByRole('banner')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual(['Home']);
    expect(screen.getByRole('main')).toBeInTheDocument();
  });
  it('HomeLayout renders two column scroll containers', () => {
    render(<HomeLayout topRow={<p>cards</p>} main={<p>chart</p>} right={<p>lists</p>} />);
    expect(screen.getByTestId('home-left')).toHaveTextContent('cardschart');
    expect(screen.getByTestId('home-right')).toHaveTextContent('lists');
  });
});
