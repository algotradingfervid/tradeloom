import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderApp } from '../../test/render';

// Labels are Go's (internal/settings/shortcuts.go, Ruling 83-F I3): "Kill switch", "Draw trend line".
const row = (label: string) => screen.findByRole('row', { name: new RegExp(`^${label}`) });

describe('FR-SHELL-019 Settings → Shortcuts', () => {
  it('lists every registry action with its scope and binding', async () => {
    renderApp({ route: '/settings/shortcuts' });
    expect(await row('Go to Home')).toHaveTextContent('Alt+1');
    expect(await row('Draw trend line')).toHaveTextContent('Chart');
    expect(await row('Kill switch')).toHaveTextContent('Ctrl+Shift+K');
    expect(await row('Scalper: buy')).toHaveTextContent('Not set');
  });
  it('AC2: a binding used in an overlapping scope is refused, naming the other action', async () => {
    const { user } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    const dialog = screen.getByRole('dialog', { name: 'Change shortcut: Kill switch' });
    expect(within(dialog).getByLabelText('New shortcut')).toHaveFocus();
    await user.keyboard('{Alt>}1{/Alt}');
    expect(within(dialog).getByLabelText('New shortcut')).toHaveTextContent('Alt+1');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Already used by Go to Home');
  });
  it('AC1 (settings part): remapping shows the new binding and the old one disappears', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    await user.click(screen.getByRole('button', { name: 'Save' }));
    expect(await row('Kill switch')).toHaveTextContent('Ctrl+Shift+X');
    expect(await row('Kill switch')).not.toHaveTextContent('Ctrl+Shift+K');
    expect(backend.callsTo('settings.rebind')[0].args).toEqual(['kill.switch', 'Ctrl+Shift+X']);
  });
  it('resets all bindings to their defaults after confirmation', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await backend.settings.rebind('kill.switch', 'Ctrl+Shift+X');
    await user.click(await screen.findByRole('button', { name: 'Reset to defaults' }));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Reset to defaults' }));
    expect(await row('Kill switch')).toHaveTextContent('Ctrl+Shift+K');
    expect(backend.callsTo('settings.resetShortcuts')).toHaveLength(1);
  });
  it('Task 6 carry-forward: a key Go does not accept (NumpadAdd, IntlBackslash) is explained and cannot be saved', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    const dialog = screen.getByRole('dialog', { name: 'Change shortcut: Kill switch' });
    const capture = within(dialog).getByLabelText('New shortcut');
    fireEvent.keyDown(capture, { key: '+', code: 'NumpadAdd', ctrlKey: true });
    expect(within(dialog).getByRole('alert')).toHaveTextContent('This key cannot be used in a shortcut');
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
    fireEvent.keyDown(capture, { key: '<', code: 'IntlBackslash', altKey: true });
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
    fireEvent.keyDown(capture, { key: 'Dead', code: 'Unidentified', altKey: true });
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeDisabled();
    // A supported key clears the explanation.
    fireEvent.keyDown(capture, { key: 'F7', code: 'F7' });
    expect(capture).toHaveTextContent('F7');
    expect(within(dialog).queryByRole('alert')).toBeNull();
    expect(within(dialog).getByRole('button', { name: 'Save' })).toBeEnabled();
    expect(backend.callsTo('settings.rebind')).toHaveLength(0);
  });
  it('shows Go\'s refusal reason for an invalid binding and keeps the dialog open', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    const dialog = screen.getByRole('dialog', { name: 'Change shortcut: Kill switch' });
    await user.keyboard('{Control>}c{/Control}');
    backend.failNext('settings.rebind', 'Ctrl+C is reserved and cannot be used', 'invalid');
    await user.click(within(dialog).getByRole('button', { name: 'Save' }));
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('Ctrl+C is reserved and cannot be used');
    expect(await row('Kill switch')).toHaveTextContent('Ctrl+Shift+K');
  });
  it('key presses in the capture box never trigger app shortcuts', async () => {
    const { user, router } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    await user.keyboard('{Alt>}1{/Alt}');
    expect(router.state.location.pathname).toBe('/settings/shortcuts');
  });
  it('Escape cancels without saving', async () => {
    const { user, backend } = renderApp({ route: '/settings/shortcuts' });
    await user.click(await screen.findByRole('button', { name: 'Change shortcut for Kill switch' }));
    await user.keyboard('{Control>}{Shift>}x{/Shift}{/Control}');
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(backend.callsTo('settings.rebind')).toHaveLength(0);
    expect(screen.getByRole('button', { name: 'Change shortcut for Kill switch' })).toHaveFocus();
  });
});
