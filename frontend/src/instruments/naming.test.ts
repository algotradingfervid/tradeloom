import { describe, expect, it } from 'vitest';
import {
  displayName, exchangeTag, instrumentId, isFuture, isInScope, itemIdentifier, itemTag, labelOfItem, OutOfScopeError, tickRupees,
} from './naming';
import { ALL_IN_SCOPE, NIFTY50, NIFTY_OPTION, RELIANCE, RELIANCE_BSE, RELIANCE_OCT_FUT, SENSEX, SENSEX_OCT_FUT } from './fixtures';

describe('instrument naming (FR-WATCH-013) and tagging (FR-WATCH-014)', () => {
  it('AC1 future: display "RELIANCE OCT FUT", F badge, identifier NFO:RELIANCE26OCTFUT', () => {
    expect(displayName(RELIANCE_OCT_FUT)).toBe('RELIANCE OCT FUT');
    expect(isFuture(RELIANCE_OCT_FUT)).toBe(true);
    expect(instrumentId(RELIANCE_OCT_FUT)).toBe('NFO:RELIANCE26OCTFUT');
    expect(exchangeTag(RELIANCE_OCT_FUT)).toBe('NFO-FUT');
  });

  it('AC2 index: NIFTY 50 tagged INDICES', () => {
    expect(displayName(NIFTY50)).toBe('NIFTY 50');
    expect(exchangeTag(NIFTY50)).toBe('INDICES');
    expect(instrumentId(NIFTY50)).toBe('NSE:NIFTY 50');
  });

  it('an F&O stock is tagged NSE on its cash listing; BSE listing tagged BSE; BFO future tagged BFO-FUT', () => {
    expect(exchangeTag(RELIANCE)).toBe('NSE');
    expect(exchangeTag(RELIANCE_BSE)).toBe('BSE');
    expect(exchangeTag(SENSEX_OCT_FUT)).toBe('BFO-FUT');
  });

  it('NFR-15: options, MCX and currency are out of scope', () => {
    expect(isInScope(NIFTY_OPTION)).toBe(false);
    expect(isInScope({ tag: 'MCX-FUT' })).toBe(false);
    expect(isInScope({ tag: 'CDS-FUT' })).toBe(false);
    expect(() => exchangeTag(NIFTY_OPTION)).toThrow(OutOfScopeError);
  });

  it('every in-scope fixture has exactly one tag from the allowed set', () => {
    for (const i of ALL_IN_SCOPE) {
      expect(['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT']).toContain(exchangeTag(i));
    }
  });

  it('tick size is carried in paise: 5 paise = 0.05 rupees', () => {
    expect(tickRupees(RELIANCE)).toBe(0.05);
    expect(tickRupees(RELIANCE_OCT_FUT)).toBe(0.1);
  });

  it('watchlist items (no tag field) map to the same tag and identifier as the instrument', () => {
    const item = { token: SENSEX.token, exchange: 'BSE', symbol: 'SENSEX', displayName: 'SENSEX', name: 'SENSEX', isIndex: true, missing: false };
    expect(itemTag(item)).toBe('INDICES');
    expect(itemIdentifier(item)).toBe('BSE:SENSEX');
    expect(itemTag({ exchange: 'NFO', isIndex: false })).toBe('NFO-FUT');
    expect(itemTag({ exchange: 'BSE', isIndex: false })).toBe('BSE');
    expect(labelOfItem(item)).toEqual({ token: SENSEX.token, tag: 'INDICES', displayName: 'SENSEX', tradingsymbol: 'SENSEX', name: 'SENSEX' });
  });

  it('labelOfItem never throws: an out-of-scope or missing item keeps its raw exchange as the tag', () => {
    const mcx = { token: 1, exchange: 'MCX', symbol: 'GOLDM', displayName: 'GOLDM', name: 'GOLD', isIndex: false };
    expect(labelOfItem(mcx)).toEqual({ token: 1, tag: 'MCX', displayName: 'GOLDM', tradingsymbol: 'GOLDM', name: 'GOLD' });
    expect(labelOfItem({ token: 2, exchange: '', symbol: '', displayName: '', name: '', isIndex: false }).tag).toBe('');
  });
});
