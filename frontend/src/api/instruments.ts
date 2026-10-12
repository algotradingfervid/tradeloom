import type { InstrumentDTO } from './backend';
import type { Instrument } from '../instruments/types';
import { EXCHANGE_TAGS, isInScope, type ExchangeTag } from '../instruments/naming';
import { BindingError, call } from './call';
import { currentBackend } from './current';

export function toInstrument(d: InstrumentDTO): Instrument {
  const i: Instrument = {
    token: d.token, exchange: d.exchange, segment: d.segment, tag: d.tag, tradingsymbol: d.tradingsymbol, name: d.name,
    displayName: d.displayName, instrumentType: d.instrumentType, expiryMs: d.expiryMs, tickSizePaise: d.tickSizePaise,
    lotSize: d.lotSize, isIndex: d.isIndex, isFuture: d.isFuture, underlying: d.underlying,
  };
  if (d.alsoOnNse) i.alsoOnNse = true; // optional flag: left out when false, so rows compare equal to the fixtures
  return i;
}

const inScope = (list: InstrumentDTO[]): Instrument[] => list.map(toInstrument).filter(isInScope);

/** Go returns [] for an empty query, so the round trip is skipped; the UI shows "Type to search". */
export async function searchInstruments(
  query: string, tags: readonly ExchangeTag[] = EXCHANGE_TAGS, limit = 50,
): Promise<Instrument[]> {
  const q = query.trim();
  if (q === '') return [];
  return inScope(await call(currentBackend().instruments.search(q, [...tags], limit)));
}

export async function instrumentsByTokens(tokens: readonly number[]): Promise<Instrument[]> {
  if (tokens.length === 0) return [];
  return inScope(await call(currentBackend().instruments.byTokens([...tokens])));
}

export async function instrumentByIdentifier(id: string): Promise<Instrument> {
  const i = toInstrument(await call(currentBackend().instruments.byIdentifier(id)));
  if (!isInScope(i)) throw new BindingError('out_of_scope', `${id} is not supported`);
  return i;
}
