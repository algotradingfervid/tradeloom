import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { focusNewestToast, pushToast } from '../store/toasts';
import { ToastRegion } from './ToastRegion';

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

const advance = (ms: number) => act(() => { vi.advanceTimersByTime(ms); });

describe('FR-SHELL-009 toasts', () => {
  it('AC1: SUCCESS disappears after 5 s ± 0.5 s', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'success', message: 'Strategy saved' }); });
    advance(4500);
    expect(screen.getByText('Strategy saved')).toBeInTheDocument();
    advance(1000);
    expect(screen.queryByText('Strategy saved')).toBeNull();
  });
  it('WARNING stays for 8 s', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'warning', message: 'Calendar may be stale' }); });
    advance(7500);
    expect(screen.getByText('Calendar may be stale')).toBeInTheDocument();
    advance(1000);
    expect(screen.queryByText('Calendar may be stale')).toBeNull();
  });
  it('AC2: ERROR is still visible after 60 s until × is clicked', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'error', message: 'Holdings could not be loaded' }); });
    advance(60_000);
    const toast = screen.getByText('Holdings could not be loaded').closest('.toast') as HTMLElement;
    fireEvent.click(within(toast).getByRole('button', { name: 'Dismiss notification' }));
    expect(screen.queryByText('Holdings could not be loaded')).toBeNull();
  });
  it('hovering pauses the timer; leaving resumes the remaining time', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'info', message: 'Found 5 instruments' }); });
    advance(4000);
    const toast = screen.getByText('Found 5 instruments').closest('.toast') as HTMLElement;
    fireEvent.mouseEnter(toast);
    advance(10_000);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    fireEvent.mouseLeave(toast);
    advance(900);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    advance(200);
    expect(screen.queryByText('Found 5 instruments')).toBeNull();
  });
  it('AC3: four toasts within 1 s → three visible (newest on top); the fourth appears when one is dismissed', () => {
    render(<ToastRegion />);
    act(() => {
      for (const m of ['one', 'two', 'three', 'four']) pushToast({ level: 'error', message: m });
    });
    const messages = () => screen.getAllByTestId('toast-message').map((n) => n.textContent);
    expect(messages()).toEqual(['three', 'two', 'one']);
    fireEvent.click(within(screen.getByText('two').closest('.toast') as HTMLElement).getByRole('button', { name: 'Dismiss notification' }));
    expect(messages()).toEqual(['four', 'three', 'one']);
  });
  it('is a labelled live region; focusNewestToast focuses the top toast', () => {
    render(<ToastRegion />);
    expect(screen.getByRole('region', { name: 'Notifications' })).toHaveAttribute('aria-live', 'polite');
    expect(focusNewestToast()).toBe(false);
    act(() => {
      pushToast({ level: 'error', message: 'older' });
      pushToast({ level: 'error', message: 'newer' });
    });
    act(() => { focusNewestToast(); });
    expect(screen.getByText('newer').closest('.toast')).toHaveFocus();
  });
  it('runs the action and dismisses the toast', () => {
    const onSelect = vi.fn();
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'warning', message: 'The app crashed last time', action: { label: 'View report', onSelect } }); });
    fireEvent.click(screen.getByRole('button', { name: 'View report' }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('The app crashed last time')).toBeNull();
  });
});

describe('toast focus (no focus theft, Alt+T round trip)', () => {
  it('a new toast never takes focus from where the user is', () => {
    render(<><input aria-label="Quantity" /><ToastRegion /></>);
    const input = screen.getByRole('textbox', { name: 'Quantity' });
    input.focus();
    act(() => {
      pushToast({ level: 'error', message: 'Holdings could not be loaded' });
      pushToast({ level: 'info', message: 'Found 5 instruments' });
    });
    expect(input).toHaveFocus();
    expect(screen.getByText('Holdings could not be loaded').closest('.toast')).toHaveAttribute('role', 'alert');
    expect(screen.getByText('Found 5 instruments').closest('.toast')).toHaveAttribute('role', 'status');
  });
  it('dismissing the focused toast moves focus to the next toast, then back to where focus was before', () => {
    render(<><input aria-label="Quantity" /><ToastRegion /></>);
    const input = screen.getByRole('textbox', { name: 'Quantity' });
    input.focus();
    act(() => {
      pushToast({ level: 'error', message: 'older' });
      pushToast({ level: 'error', message: 'newer' });
    });
    act(() => { focusNewestToast(); });
    const dismissOf = (m: string) => within(screen.getByText(m).closest('.toast') as HTMLElement).getByRole('button', { name: 'Dismiss notification' });
    fireEvent.click(dismissOf('newer'));
    expect(screen.getByText('older').closest('.toast')).toHaveFocus();
    fireEvent.click(dismissOf('older'));
    expect(input).toHaveFocus();
  });
});
