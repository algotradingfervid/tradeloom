import type { Instrument } from './types';

export const EXCHANGE_TAGS = ['INDICES', 'NSE', 'BSE', 'NFO-FUT', 'BFO-FUT'] as const;
export type ExchangeTag = (typeof EXCHANGE_TAGS)[number];

export class OutOfScopeError extends Error {
  constructor(tag: string) {
    super(`tag ${tag} is out of scope (NFR-15)`);
    this.name = 'OutOfScopeError';
  }
}

/** What InstrumentLabel / InstrumentRow need; a full Instrument and a watchlist item both provide it. */
export type LabelInstrument = Pick<Instrument, 'token' | 'tag' | 'displayName' | 'tradingsymbol' | 'name'>;

/** NFR-15 guard: true only for indices, NSE/BSE cash and NFO/BFO futures. */
export function isInScope(i: Pick<Instrument, 'tag'>): boolean {
  return (EXCHANGE_TAGS as readonly string[]).includes(i.tag);
}

/** FR-WATCH-014: the tag is the instrument's own master tag (P0-2 instruments.Tag), never inferred from context. */
export function exchangeTag(i: Pick<Instrument, 'tag'>): ExchangeTag {
  if (!isInScope(i)) throw new OutOfScopeError(i.tag);
  return i.tag as ExchangeTag;
}

export function isFuture(i: Pick<Instrument, 'tag'>): boolean {
  return i.tag === 'NFO-FUT' || i.tag === 'BFO-FUT';
}

/** Tick size in rupees (the DTO carries integer paise). */
export function tickRupees(i: Pick<Instrument, 'tickSizePaise'>): number {
  return i.tickSizePaise / 100;
}

/**
 * Watchlist items (service.WatchlistItem) carry exchange + isIndex, not the tag. Inside the NFR-15 scope the
 * mapping is exact: indices → INDICES, NFO/BFO rows are futures, NSE/BSE rows are cash.
 */
export function itemTag(item: { exchange: string; isIndex: boolean }): ExchangeTag {
  if (item.isIndex) return 'INDICES';
  if (item.exchange === 'NFO') return 'NFO-FUT';
  if (item.exchange === 'BFO') return 'BFO-FUT';
  if (item.exchange === 'NSE' || item.exchange === 'BSE') return item.exchange;
  throw new OutOfScopeError(item.exchange);
}

export function itemIdentifier(item: { exchange: string; symbol: string }): string {
  return `${item.exchange}:${item.symbol}`;
}

/**
 * Never throws and never drops the item (P0-3 Ruling 38, P0-5 Ruling 115 m3): an item outside the NFR-15 scope keeps
 * its raw exchange as the tag, so InstrumentLabel shows it with the "View only" flag. A missing item (Go sends no
 * exchange or symbol) gets an empty tag; callers check `item.missing` first.
 */
export function labelOfItem(item: { token: number; exchange: string; symbol: string; displayName: string; name: string; isIndex: boolean }): LabelInstrument {
  let tag: string;
  try {
    tag = itemTag(item);
  } catch (e) {
    if (!(e instanceof OutOfScopeError)) throw e;
    tag = item.exchange;
  }
  return { token: item.token, tag, displayName: item.displayName, tradingsymbol: item.symbol, name: item.name };
}

/** FR-WATCH-013: Kite EXCHANGE:TRADINGSYMBOL form, used internally and in deep links. */
export function instrumentId(i: Pick<Instrument, 'exchange' | 'tradingsymbol'>): string {
  return `${i.exchange}:${i.tradingsymbol}`;
}

/** FR-WATCH-013: the one name formatter. Go derives displayName at import; the symbol is the fallback. */
export function displayName(i: Pick<Instrument, 'displayName' | 'tradingsymbol'>): string {
  return i.displayName || i.tradingsymbol;
}
