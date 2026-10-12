import { describe, expect, it } from 'vitest';
import { nextWatchlistName, validateWatchlistName } from './names';

describe('watchlist names (FR-WATCH-002)', () => {
  it('AC1: lowest unused "Watchlist N"', () => {
    expect(nextWatchlistName(['Watchlist 1', 'Watchlist 2'])).toBe('Watchlist 3');
    expect(nextWatchlistName(['Watchlist 1', 'watchlist 3'])).toBe('Watchlist 2');
    expect(nextWatchlistName([])).toBe('Watchlist 1');
  });
  it('AC2/AC3: trimmed, 1–40 chars, unique case-insensitively', () => {
    expect(validateWatchlistName('   ', [])).toBe('Enter a name');
    expect(validateWatchlistName('x'.repeat(41), [])).toBe('Use 40 characters or fewer');
    expect(validateWatchlistName(' watchlist 1 ', ['Watchlist 1'])).toBe('A watchlist with this name exists');
    expect(validateWatchlistName('Banks', ['Watchlist 1'])).toBeNull();
  });
  it('counts characters as Go counts runes (utf8.RuneCountInString), not UTF-16 units', () => {
    expect(validateWatchlistName('📈'.repeat(40), [])).toBeNull();
    expect(validateWatchlistName('📈'.repeat(41), [])).toBe('Use 40 characters or fewer');
  });
});
