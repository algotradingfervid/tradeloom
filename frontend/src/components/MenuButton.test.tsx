import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { MenuButton, type MenuItem } from './MenuButton';

function setup() {
  const onProfile = vi.fn();
  const onDark = vi.fn();
  const items: MenuItem[] = [
    { kind: 'static', id: 'who', content: <strong>Asha Rao</strong> },
    { kind: 'separator', id: 's1' },
    { kind: 'item', id: 'profile', label: 'Profile', onSelect: onProfile },
    { kind: 'item', id: 'help', label: 'Help', onSelect: vi.fn() },
    { kind: 'checkbox', id: 'dark', label: 'Dark theme', checked: false, onChange: onDark },
  ];
  render(<><MenuButton label="Account menu" trigger={<span>AR</span>} items={items} /><button>outside</button></>);
  return { onProfile, onDark, user: userEvent.setup() };
}

describe('MenuButton (FR-SHELL-010 keyboard behaviour)', () => {
  it('opens with focus on the first item; arrows cycle; Escape closes and refocuses the trigger', async () => {
    const { user } = setup();
    const trigger = screen.getByRole('button', { name: 'Account menu' });
    await user.click(trigger);
    expect(screen.getByRole('menuitem', { name: 'Profile' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    expect(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('menuitem', { name: 'Profile' })).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    expect(trigger).toHaveFocus();
  });
  it('activates items with Enter and closes', async () => {
    const { user, onProfile } = setup();
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.keyboard('{Enter}');
    expect(onProfile).toHaveBeenCalledTimes(1);
    expect(screen.queryByRole('menu')).toBeNull();
  });
  it('reports checkbox changes and closes on outside click', async () => {
    const { user, onDark } = setup();
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.click(screen.getByRole('menuitemcheckbox', { name: 'Dark theme' }));
    expect(onDark).toHaveBeenCalledWith(true);
    await user.click(screen.getByRole('button', { name: 'Account menu' }));
    await user.click(screen.getByText('outside'));
    expect(screen.queryByRole('menu')).toBeNull();
  });
});
