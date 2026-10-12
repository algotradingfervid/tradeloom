import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { BackgroundErrorNotice } from './BackgroundErrorNotice';

const message = (r: string) => `Showing the last loaded data. The latest read failed: ${r}`;

describe('BackgroundErrorNotice (Ruling 105)', () => {
  it('M2: the role="status" live region is always mounted; only its contents toggle', () => {
    const { rerender } = render(<BackgroundErrorNotice error={null} seq={0} message={message} onRetry={() => Promise.resolve()} />);
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
    const region = screen.getByRole('status');
    rerender(<BackgroundErrorNotice error="Network unavailable" seq={1} message={message} onRetry={() => Promise.resolve()} />);
    expect(screen.getByRole('status')).toBe(region);
    expect(region).toHaveTextContent(message('Network unavailable'));
  });
  it('M1: an identical repeated failure is announced again (new text node)', () => {
    const onRetry = () => Promise.resolve();
    const { rerender } = render(<BackgroundErrorNotice error="Network unavailable" seq={1} message={message} onRetry={onRetry} />);
    const first = screen.getByText(message('Network unavailable'));
    rerender(<BackgroundErrorNotice error="Network unavailable" seq={2} message={message} onRetry={onRetry} />);
    const second = screen.getByText(message('Network unavailable'));
    expect(second).not.toBe(first);
    expect(screen.getAllByText(message('Network unavailable'))).toHaveLength(1);
  });
  it('M1: Retry is aria-disabled until its promise settles and ignores clicks meanwhile', async () => {
    const user = userEvent.setup();
    let settle!: () => void;
    const onRetry = vi.fn(() => new Promise<void>((r) => { settle = r; }));
    render(<BackgroundErrorNotice error="Network unavailable" seq={1} message={message} onRetry={onRetry} />);
    const retry = screen.getByRole('button', { name: 'Retry' });
    expect(retry).not.toHaveAttribute('aria-disabled');
    await user.click(retry);
    expect(retry).toHaveAttribute('aria-disabled', 'true');
    await user.click(retry);
    expect(onRetry).toHaveBeenCalledTimes(1);
    await act(async () => settle());
    expect(retry).not.toHaveAttribute('aria-disabled');
  });
});
