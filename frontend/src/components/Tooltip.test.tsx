import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { IconButton } from './IconButton';

afterEach(() => vi.useRealTimers());

describe('FR-SHELL-013 tooltips', () => {
  it('AC1: an icon-only button exposes its label as the accessible name', () => {
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    expect(screen.getByRole('button', { name: 'Refresh chart data' })).toBeInTheDocument();
  });
  it('appears after 500 ms of hover, not before', () => {
    vi.useFakeTimers();
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    fireEvent.mouseEnter(screen.getByRole('button'));
    act(() => vi.advanceTimersByTime(499));
    expect(screen.queryByRole('tooltip')).toBeNull();
    act(() => vi.advanceTimersByTime(1));
    expect(screen.getByRole('tooltip')).toHaveTextContent('Refresh chart data');
    fireEvent.mouseLeave(screen.getByRole('button'));
    expect(screen.queryByRole('tooltip')).toBeNull();
  });
  it('AC2: appears immediately on keyboard focus and hides on Escape', () => {
    render(<IconButton label="Refresh chart data" icon="refresh" />);
    const button = screen.getByRole('button');
    act(() => button.focus());
    expect(screen.getByRole('tooltip')).toHaveTextContent('Refresh chart data');
    expect(button).toHaveAccessibleDescription('Refresh chart data');
    fireEvent.keyDown(button, { key: 'Escape' });
    expect(screen.queryByRole('tooltip')).toBeNull();
  });
});
