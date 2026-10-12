import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { focusNewestToast, pushToast, useToastStore } from '../store/toasts';
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

describe('fix round 1 (Ruling 86)', () => {
  const Page = () => (
    <>
      <main id="main-content"><h1>Home</h1><input aria-label="A" /><input aria-label="B" /></main>
      <ToastRegion />
    </>
  );
  const toastOf = (m: string) => screen.getByText(m).closest('.toast') as HTMLElement;
  const dismissOf = (m: string) => within(toastOf(m)).getByRole('button', { name: 'Dismiss notification' });
  const queuedMessages = () => useToastStore.getState().queued.map((t) => t.message);

  it('M1: leaving the toasts forgets the Alt+T origin; closing the last focused toast falls back to the view heading', () => {
    render(<Page />);
    screen.getByRole('textbox', { name: 'A' }).focus();
    act(() => { pushToast({ level: 'error', message: 'only' }); });
    act(() => { focusNewestToast(); });
    act(() => { screen.getByRole('textbox', { name: 'B' }).focus(); });
    act(() => { toastOf('only').focus(); });
    fireEvent.click(dismissOf('only'));
    expect(screen.getByRole('heading', { name: 'Home' })).toHaveFocus();
  });
  it('M1: with no Alt+T origin, closing a focused toast focuses the view heading, not body', () => {
    render(<Page />);
    act(() => { pushToast({ level: 'error', message: 'only' }); });
    act(() => { toastOf('only').focus(); });
    fireEvent.click(dismissOf('only'));
    expect(screen.getByRole('heading', { name: 'Home' })).toHaveFocus();
  });
  it('M2: an action closes its toast without handing focus to another toast', () => {
    const onSelect = vi.fn();
    render(<Page />);
    act(() => {
      pushToast({ level: 'error', message: 'older' });
      pushToast({ level: 'warning', message: 'newer', action: { label: 'Open', onSelect } });
    });
    act(() => { focusNewestToast(); });
    fireEvent.click(screen.getByRole('button', { name: 'Open' }));
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(screen.queryByText('newer')).toBeNull();
    expect(toastOf('older')).not.toHaveFocus();
  });
  it('M3: the queue holds at most 10 toasts and drops the oldest non-error first', () => {
    render(<ToastRegion />);
    act(() => {
      for (const m of ['e1', 'e2', 'e3', 'e4']) pushToast({ level: 'error', message: m });
      for (let i = 1; i <= 10; i++) pushToast({ level: 'info', message: `i${i}` });
    });
    expect(queuedMessages()).toEqual(['e4', 'i2', 'i3', 'i4', 'i5', 'i6', 'i7', 'i8', 'i9', 'i10']);
  });
  it('M3: a queued info/success toast older than its duration is dropped instead of shown', () => {
    render(<ToastRegion />);
    act(() => {
      for (const m of ['e1', 'e2', 'e3']) pushToast({ level: 'error', message: m });
      pushToast({ level: 'success', message: 'stale' });
    });
    advance(6000);
    act(() => { pushToast({ level: 'warning', message: 'fresh' }); });
    fireEvent.click(dismissOf('e2'));
    expect(screen.queryByText('stale')).toBeNull();
    expect(screen.getByText('fresh')).toBeInTheDocument();
    expect(queuedMessages()).toEqual([]);
  });
  it('M4: a toast with an action stays until it is dismissed', () => {
    render(<ToastRegion />);
    act(() => { pushToast({ level: 'info', message: 'Update ready', action: { label: 'Open', onSelect: () => {} } }); });
    advance(60_000);
    expect(screen.getByText('Update ready')).toBeInTheDocument();
  });
  it('M7: unmounting with a running timer leaves no timer behind', () => {
    const { unmount } = render(<ToastRegion />);
    act(() => { pushToast({ level: 'info', message: 'Found 5 instruments' }); });
    advance(1000);
    unmount();
    expect(vi.getTimerCount()).toBe(0);
    advance(10_000);
    expect(useToastStore.getState().visible.map((t) => t.message)).toEqual(['Found 5 instruments']);
  });
  it('M7: a fifth toast waits in the queue', () => {
    render(<ToastRegion />);
    act(() => {
      for (const m of ['one', 'two', 'three', 'four', 'five']) pushToast({ level: 'info', message: m });
    });
    expect(screen.getAllByTestId('toast-message').map((n) => n.textContent)).toEqual(['three', 'two', 'one']);
    expect(queuedMessages()).toEqual(['four', 'five']);
  });
  it('M7: a focused toast keeps its timer paused until focus leaves', () => {
    render(<Page />);
    act(() => { pushToast({ level: 'info', message: 'Found 5 instruments' }); });
    act(() => { focusNewestToast(); });
    advance(10_000);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    act(() => { screen.getByRole('textbox', { name: 'A' }).focus(); });
    advance(4900);
    expect(screen.getByText('Found 5 instruments')).toBeInTheDocument();
    advance(200);
    expect(screen.queryByText('Found 5 instruments')).toBeNull();
  });
});
