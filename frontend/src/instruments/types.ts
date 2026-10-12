/**
 * Frontend view of an instrument-master row (FR-WATCH-008). Field-for-field the canonical
 * service.InstrumentDTO JSON (contract §C); src/api/instruments.ts copies DTOs into it.
 */
export interface Instrument {
  token: number;
  exchange: string; // NSE | BSE | NFO | BFO
  segment: string; // Kite segment, e.g. NSE, INDICES, NFO-FUT
  tag: string; // P0-2 instruments.Tag: INDICES | NSE | BSE | NFO-FUT | BFO-FUT (anything else is out of scope)
  tradingsymbol: string;
  name: string;
  displayName: string; // derived by Go at import (ARCHITECTURE §5.7)
  instrumentType: string; // EQ | FUT
  expiryMs: number; // 0 = no expiry
  tickSizePaise: number; // integer paise, e.g. 5 = 0.05 rupees
  lotSize: number;
  isIndex: boolean;
  isFuture: boolean;
  underlying: string; // futures: the underlying symbol; '' otherwise
}
