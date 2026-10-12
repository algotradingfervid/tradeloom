import { act, fireEvent, render, screen, within } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('../../api/instruments', () => ({ searchInstruments: vi.fn(), instrumentByIdentifier: vi.fn(), instrumentsByTokens: vi.fn(async () => []) }));
import { instrumentByIdentifier, searchInstruments } from '../../api/instruments';
import { BindingError } from '../../api/call';
import { SEARCH_DEBOUNCE_MS, SymbolSearchDialog } from './SymbolSearchDialog';
import { BANKNIFTY_OCT_FUT, NIFTY_OPTION, NIFTYBANK, RELIANCE, RELIANCE_OCT_FUT, SBIN, SENSEX } from '../../instruments/fixtures';
import type { Instrument } from '../../instruments/types';
import { useInstruments } from '../../store/instrumentsStore';
import { resetOnLogout } from '../../store/reset';

/** Runs due timers and the promise callbacks they start, inside act. */
const flush = (ms = SEARCH_DEBOUNCE_MS) => act(async () => { await vi.advanceTimersByTimeAsync(ms); });

const setup = () => {
  const onClose = vi.fn(); const onSelect = vi.fn();
  const view = render(<SymbolSearchDialog open onClose={onClose} onSelect={onSelect} />);
  return { onClose, onSelect, view, input: screen.getByRole('combobox', { name: 'Search instruments' }) as HTMLInputElement };
};

const type = async (input: HTMLInputElement, value: string) => {
  fireEvent.change(input, { target: { value } });
  await flush();
};

function deferred<T>() {
  let resolve!: (v: T) => void; let reject!: (e: unknown) => void;
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  vi.mocked(searchInstruments).mockImplementation(async (q: string) => (q === 'r' ? [RELIANCE, SBIN] : []));
});
afterEach(() => { vi.useRealTimers(); });

describe('SymbolSearchDialog', () => {
  it('AC1: input focused, 4 quick chips, "Type to search" until the user types (no empty-query call)', async () => {
    const { input } = setup();
    expect(input).toHaveFocus();
    expect(screen.getByRole('dialog', { name: 'Search instruments' })).toBeInTheDocument();
    for (const chip of ['NIFTY 50', 'NIFTY BANK', 'SENSEX', 'BANKEX']) expect(screen.getByRole('button', { name: chip })).toBeInTheDocument();
    expect(input).toHaveAttribute('placeholder', 'Search, e.g. SBIN, TCS');
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await flush(200);
    expect(searchInstruments).not.toHaveBeenCalled();
    await type(input, '   ');
    expect(searchInstruments).not.toHaveBeenCalled();
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    await type(input, 'r');
    expect(screen.getByText('RELIANCE')).toBeInTheDocument();
    expect(searchInstruments).toHaveBeenCalledWith('r', ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'], 50);
    expect(screen.queryByText('Type to search')).toBeNull();
  });

  it('debounces typing: one call for the last query, none before the debounce elapses', async () => {
    const { input } = setup();
    fireEvent.change(input, { target: { value: 'r' } });
    await flush(SEARCH_DEBOUNCE_MS - 20);
    fireEvent.change(input, { target: { value: 're' } });
    await flush(SEARCH_DEBOUNCE_MS - 20);
    fireEvent.change(input, { target: { value: 'rel' } });
    await flush(SEARCH_DEBOUNCE_MS - 1);
    expect(searchInstruments).not.toHaveBeenCalled();
    await flush(1);
    expect(searchInstruments).toHaveBeenCalledTimes(1);
    expect(searchInstruments).toHaveBeenCalledWith('rel', expect.anything(), 50);
  });

  it('AC2: BANKNIFTY lists the index and the future', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([NIFTYBANK, BANKNIFTY_OCT_FUT]);
    const { input } = setup();
    await type(input, 'BANKNIFTY');
    const list = screen.getByRole('listbox');
    expect(within(list).getByText('NIFTY BANK')).toBeInTheDocument();
    expect(within(list).getByText('BANKNIFTY OCT FUT')).toBeInTheDocument();
    expect(within(list).queryByText(/ CE$| PE$/)).toBeNull();
  });

  it('NFR-15: an out-of-scope row is never offered or cached', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([NIFTY_OPTION, SBIN]);
    const { input } = setup();
    await type(input, 'nifty');
    expect(screen.getAllByRole('option')).toHaveLength(1);
    expect(screen.queryByText(NIFTY_OPTION.displayName)).toBeNull();
    expect(useInstruments.getState().byToken[NIFTY_OPTION.token]).toBeUndefined();
  });

  it('AC3: "reliance oct fut" shows the future with an F badge', async () => {
    vi.mocked(searchInstruments).mockResolvedValue([RELIANCE_OCT_FUT]);
    const { input } = setup();
    await type(input, 'reliance oct fut');
    const opt = screen.getByRole('option', { name: /RELIANCE OCT FUT/ });
    expect(within(opt).getByLabelText('Future')).toBeInTheDocument();
  });

  it('combobox/listbox pattern: active descendant follows the arrows; the result count is announced politely', async () => {
    const { input } = setup();
    expect(input).toHaveAttribute('aria-expanded', 'false');
    await type(input, 'r');
    const list = screen.getByRole('listbox');
    expect(input).toHaveAttribute('aria-controls', list.id);
    expect(input).toHaveAttribute('aria-expanded', 'true');
    const [first, second] = within(list).getAllByRole('option');
    expect(input).toHaveAttribute('aria-activedescendant', first.id);
    expect(first).toHaveAttribute('aria-selected', 'true');
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    expect(input).toHaveAttribute('aria-activedescendant', second.id);
    expect(second).toHaveAttribute('aria-selected', 'true');
    expect(first).toHaveAttribute('aria-selected', 'false');
    fireEvent.keyDown(input, { key: 'ArrowDown' }); // stays on the last row
    expect(input).toHaveAttribute('aria-activedescendant', second.id);
    fireEvent.keyDown(input, { key: 'ArrowUp' });
    fireEvent.keyDown(input, { key: 'ArrowUp' }); // stays on the first row
    expect(input).toHaveAttribute('aria-activedescendant', first.id);
    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveTextContent('2 results');
    await type(input, 'zzz');
    expect(status).toHaveTextContent('No instruments match');
    // Shown to sighted users too; that copy is aria-hidden so the status region is the only announcement.
    const visible = screen.getAllByText('No instruments match').filter((el) => el !== status);
    expect(visible).toHaveLength(1);
    expect(visible[0]).toHaveAttribute('aria-hidden', 'true');
  });

  it('AC4: ArrowDown + Enter selects, caches and closes', async () => {
    const { input, onSelect, onClose } = setup();
    await type(input, 'r');
    fireEvent.keyDown(input, { key: 'ArrowDown' });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).toHaveBeenCalledWith(SBIN);
    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalled();
    expect(useInstruments.getState().byToken[SBIN.token]).toEqual(SBIN);
  });

  it('a click on a row selects it', async () => {
    const { input, onSelect, onClose } = setup();
    await type(input, 'r');
    fireEvent.click(screen.getByRole('option', { name: /RELIANCE/ }));
    expect(onSelect).toHaveBeenCalledWith(RELIANCE);
    expect(onClose).toHaveBeenCalled();
  });

  it('Enter does nothing while the shown rows belong to an older query', async () => {
    const { input, onSelect } = setup();
    await type(input, 'r');
    fireEvent.change(input, { target: { value: 'sb' } }); // debounce still pending
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('quick chip resolves the index and selects it', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(SENSEX);
    const { onSelect, onClose } = setup();
    fireEvent.click(screen.getByRole('button', { name: 'SENSEX' }));
    await flush(0);
    expect(onSelect).toHaveBeenCalledWith(SENSEX);
    expect(onClose).toHaveBeenCalled();
    expect(instrumentByIdentifier).toHaveBeenCalledWith('BSE:SENSEX');
  });

  it('a failed chip lookup shows the reason and keeps the dialog open', async () => {
    vi.mocked(instrumentByIdentifier).mockRejectedValue(new BindingError('not_found', 'BSE:BANKEX not found'));
    const { onSelect, onClose } = setup();
    fireEvent.click(screen.getByRole('button', { name: 'BANKEX' }));
    await flush(0);
    expect(screen.getByRole('alert')).toHaveTextContent('BSE:BANKEX not found');
    expect(onSelect).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });

  it('a transport/binding failure (Go\'s Search itself never refuses) shows the reason and hides the previous rows; the next success clears it', async () => {
    const { input } = setup();
    await type(input, 'r');
    vi.mocked(searchInstruments).mockRejectedValueOnce(new BindingError('internal', 'Bridge unavailable'));
    await type(input, 'x');
    expect(screen.getByRole('alert')).toHaveTextContent('Bridge unavailable');
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    await type(input, 'r');
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('SBIN')).toBeInTheDocument();
  });

  it('AC5: Escape closes without selecting', async () => {
    const { input, onSelect, onClose } = setup();
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('focus returns to the opener when the dialog closes', async () => {
    function Host() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>Symbol</button>
          <SymbolSearchDialog open={open} onClose={() => setOpen(false)} onSelect={() => undefined} />
        </>
      );
    }
    render(<Host />);
    const opener = screen.getByRole('button', { name: 'Symbol' });
    opener.focus();
    fireEvent.click(opener);
    const input = screen.getByRole('combobox', { name: 'Search instruments' });
    expect(input).toHaveFocus();
    fireEvent.keyDown(input, { key: 'Escape' });
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it('a slow earlier query never overwrites a newer one', async () => {
    const slow = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockImplementation((q: string) =>
      q === 'r' ? slow.promise : Promise.resolve(q === 'sb' ? [SBIN] : []));
    const { input } = setup();
    await type(input, 'r');
    expect(searchInstruments).toHaveBeenCalledWith('r', expect.anything(), 50);
    await type(input, 'sb');
    expect(screen.getByText('SBIN')).toBeInTheDocument();
    await act(async () => { slow.resolve([RELIANCE]); });
    await flush(20);
    expect(screen.queryByText('RELIANCE')).toBeNull();
    expect(screen.getAllByRole('option')).toHaveLength(1);
  });

  it('calls that finish after the dialog closes resolve quietly (no select, no error)', async () => {
    const search = deferred<Instrument[]>();
    const chip = deferred<Instrument>();
    vi.mocked(searchInstruments).mockReturnValue(search.promise);
    vi.mocked(instrumentByIdentifier).mockReturnValue(chip.promise);
    const { input, onSelect, view } = setup();
    await type(input, 'r');
    fireEvent.click(screen.getByRole('button', { name: 'NIFTY 50' }));
    view.rerender(<SymbolSearchDialog open={false} onClose={vi.fn()} onSelect={onSelect} />);
    await act(async () => { search.reject(new BindingError('offline', 'late')); chip.resolve(SENSEX); });
    await flush(0);
    expect(onSelect).not.toHaveBeenCalled();
    view.rerender(<SymbolSearchDialog open onClose={vi.fn()} onSelect={onSelect} />);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByText('Type to search')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: 'Search instruments' })).toHaveValue('');
  });

  it('I1: the polite count region is always rendered (never display:none while empty) and visually hidden', async () => {
    const { input } = setup();
    const status = screen.getByRole('status');
    expect(status).toHaveAttribute('aria-live', 'polite');
    expect(status).toHaveClass('visually-hidden');
    expect(status).not.toHaveClass('symbol-search__hint');
    expect(status).toBeEmptyDOMElement();
    await type(input, 'r');
    expect(screen.getByRole('status')).toBe(status);
    expect(status).toHaveTextContent('2 results');
  });

  it('I2: answers that arrive after a reset (logout or user switch) select nothing, show nothing and cache nothing', async () => {
    const search = deferred<Instrument[]>();
    const chip = deferred<Instrument>();
    vi.mocked(searchInstruments).mockReturnValue(search.promise);
    vi.mocked(instrumentByIdentifier).mockReturnValue(chip.promise);
    const { input, onSelect } = setup();
    await type(input, 'r');
    fireEvent.click(screen.getByRole('button', { name: 'SENSEX' }));
    act(() => resetOnLogout());
    expect(input).toHaveValue('');
    await act(async () => { search.resolve([RELIANCE, SBIN]); chip.resolve(SENSEX); });
    await flush(0);
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(useInstruments.getState().byToken[SENSEX.token]).toBeUndefined();
    expect(useInstruments.getState().byToken[RELIANCE.token]).toBeUndefined();
    expect(screen.getByText('Type to search')).toBeInTheDocument();
  });

  it('I2: rows shown before a reset are cleared, so Enter after the reset selects nothing', async () => {
    const { input, onSelect } = setup();
    await type(input, 'r');
    act(() => resetOnLogout());
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).not.toHaveBeenCalled();
    expect(screen.queryAllByRole('option')).toHaveLength(0);
  });

  it('a chip selection is not cached again by the dialog (resolveIdentifier caches it while current)', async () => {
    vi.mocked(instrumentByIdentifier).mockResolvedValue(SENSEX);
    const writes = vi.fn();
    const unsubscribe = useInstruments.subscribe(writes);
    const { onSelect } = setup();
    fireEvent.click(screen.getByRole('button', { name: 'SENSEX' }));
    await flush(0);
    expect(onSelect).toHaveBeenCalledWith(SENSEX);
    expect(writes).toHaveBeenCalledTimes(1); // resolveIdentifier's own put only
    expect(useInstruments.getState().byToken[SENSEX.token]).toEqual(SENSEX);
    unsubscribe();
  });

  it('unmount during the debounce leaves no timer and sends nothing', async () => {
    const { input, view } = setup();
    fireEvent.change(input, { target: { value: 'r' } });
    await flush(SEARCH_DEBOUNCE_MS / 2);
    view.unmount();
    expect(vi.getTimerCount()).toBe(0);
    await flush(1_000);
    expect(searchInstruments).not.toHaveBeenCalled();
  });

  it('Enter is ignored while the request is in flight after the debounce; the listbox is marked busy', async () => {
    const { input, onSelect } = setup();
    await type(input, 'r');
    const pending = deferred<Instrument[]>();
    vi.mocked(searchInstruments).mockReturnValue(pending.promise);
    await type(input, 'sb');
    expect(searchInstruments).toHaveBeenLastCalledWith('sb', expect.anything(), 50);
    expect(screen.getByRole('listbox')).toHaveAttribute('aria-busy', 'true');
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).not.toHaveBeenCalled();
    await act(async () => { pending.resolve([SBIN]); });
    expect(screen.getByRole('listbox')).not.toHaveAttribute('aria-busy');
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(onSelect).toHaveBeenCalledWith(SBIN);
  });

  it('Enter during IME composition does not select', async () => {
    const { input, onSelect } = setup();
    await type(input, 'r');
    fireEvent.keyDown(input, { key: 'Enter', isComposing: true });
    fireEvent.keyDown(input, { key: 'Enter', keyCode: 229 });
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('hover highlighting follows mouse movement', async () => {
    const { input } = setup();
    await type(input, 'r');
    const [, second] = screen.getAllByRole('option');
    fireEvent.mouseMove(second);
    expect(second).toHaveAttribute('aria-selected', 'true');
    expect(input).toHaveAttribute('aria-activedescendant', second.id);
  });

  it('G-0: offers no order actions', async () => {
    const { input } = setup();
    await type(input, 'r');
    expect(screen.queryByRole('button', { name: /buy|sell|order/i })).toBeNull();
  });
});
