import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

function renderSidebar(collapsed = false) {
  const onToggle = vi.fn();
  const router = createMemoryRouter(
    [{ path: '*', element: (<><Sidebar entries={visibleNavEntries(6)} collapsed={collapsed} onToggleCollapsed={onToggle} /><Where /></>) }],
    { initialEntries: ['/home'] },
  );
  render(<RouterProvider router={router} />);
  return { onToggle };
}

describe('FR-SHELL-002 sidebar', () => {
  it('lists the entries with icon + label and marks the current one', () => {
    renderSidebar();
    expect(screen.getAllByRole('link')).toHaveLength(8);
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Scalper' })).not.toHaveAttribute('aria-current');
  });
  it('AC3 Up/Down move focus between entries and Enter navigates', async () => {
    const user = userEvent.setup();
    renderSidebar();
    const links = screen.getAllByRole('link');
    links[0].focus();
    await user.keyboard('{ArrowDown}');
    expect(links[1]).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}');
    expect(links[7]).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(screen.getByTestId('where')).toHaveTextContent('/scanner-alerts');
  });
  it('collapse button reports the toggle and keeps accessible names when collapsed', async () => {
    const user = userEvent.setup();
    const { onToggle } = renderSidebar(true);
    expect(screen.getByRole('link', { name: 'Home' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Expand sidebar' }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
