import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { Button } from './Button';
import { ConfirmDialog } from './ConfirmDialog';
import { Dialog } from './Dialog';
import { TextField } from './TextField';

function Host() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button onClick={() => setOpen(true)}>Open</button>
      <Dialog open={open} title="Rename" onClose={() => setOpen(false)} footer={<Button onClick={() => setOpen(false)}>Cancel</Button>}>
        <TextField label="Name" defaultValue="Main" />
      </Dialog>
    </>
  );
}

describe('FR-SHELL-015 dialogs', () => {
  it('AC2: Escape closes the dialog and focus returns to the opener', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    expect(screen.getByRole('dialog', { name: 'Rename' })).toHaveAttribute('aria-modal', 'true');
    expect(screen.getByLabelText('Name')).toHaveFocus();
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.getByText('Open')).toHaveFocus();
  });
  it('AC3: always renders a visible close × button', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(screen.queryByRole('dialog')).toBeNull();
  });
  it('traps Tab and Shift+Tab inside the dialog', async () => {
    const user = userEvent.setup();
    render(<Host />);
    await user.click(screen.getByText('Open'));
    const dialog = screen.getByRole('dialog');
    await user.tab();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Close' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  });
  it('keeps focus inside when the focused control disappears', async () => {
    function Shrinking() {
      const [show, setShow] = useState(true);
      return (
        <Dialog open title="Status" onClose={() => {}}>
          <p>details</p>
          {show && <button onClick={() => setShow(false)}>Vanish</button>}
        </Dialog>
      );
    }
    const user = userEvent.setup();
    render(<Shrinking />);
    await user.click(screen.getByText('Vanish'));
    await waitFor(() => expect(screen.getByRole('dialog')).toHaveFocus());
  });
  it('ConfirmDialog enables confirm only after the exact typed phrase; Escape cancels', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    const onCancel = vi.fn();
    render(<ConfirmDialog open title="Restore backup" message="Replaces your data." confirmLabel="Restore" typedConfirmation="RESTORE" onConfirm={onConfirm} onCancel={onCancel} />);
    const confirm = screen.getByRole('button', { name: 'Restore' });
    expect(confirm).toBeDisabled();
    await user.type(screen.getByLabelText('Type RESTORE to confirm'), 'restore');
    expect(confirm).toBeDisabled();
    await user.clear(screen.getByLabelText('Type RESTORE to confirm'));
    await user.type(screen.getByLabelText('Type RESTORE to confirm'), 'RESTORE');
    await user.click(confirm);
    expect(onConfirm).toHaveBeenCalledTimes(1);
    await user.keyboard('{Escape}');
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
