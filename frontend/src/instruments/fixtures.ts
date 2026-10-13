import type { Instrument } from './types';

const eq = (token: number, exchange: 'NSE' | 'BSE', symbol: string, name: string): Instrument => ({
  token, exchange, segment: exchange, tag: exchange, tradingsymbol: symbol, name, displayName: symbol,
  instrumentType: 'EQ', expiryMs: 0, tickSizePaise: 5, lotSize: 1, isIndex: false, isFuture: false, underlying: '',
});
const index = (token: number, exchange: 'NSE' | 'BSE', symbol: string): Instrument => ({
  token, exchange, segment: 'INDICES', tag: 'INDICES', tradingsymbol: symbol, name: symbol, displayName: symbol,
  instrumentType: 'EQ', expiryMs: 0, tickSizePaise: 5, lotSize: 0, isIndex: true, isFuture: false, underlying: '',
});
// 27 Oct 2026 15:30 IST — fixture expiry only.
const OCT_2026_EXPIRY = Date.UTC(2026, 9, 27, 10, 0);
const fut = (token: number, exchange: 'NFO' | 'BFO', symbol: string, underlying: string, lot: number): Instrument => ({
  token, exchange, segment: `${exchange}-FUT`, tag: `${exchange}-FUT`, tradingsymbol: symbol, name: underlying,
  displayName: `${underlying} OCT FUT`, instrumentType: 'FUT', expiryMs: OCT_2026_EXPIRY,
  tickSizePaise: exchange === 'NFO' ? 10 : 5, lotSize: lot, isIndex: false, isFuture: true, underlying,
});

export const NIFTY50 = index(256265, 'NSE', 'NIFTY 50');
export const NIFTYBANK = index(260105, 'NSE', 'NIFTY BANK');
export const SENSEX = index(265, 'BSE', 'SENSEX');
export const BANKEX = index(274441, 'BSE', 'BANKEX');
export const RELIANCE = eq(738561, 'NSE', 'RELIANCE', 'RELIANCE INDUSTRIES');
export const RELIANCE_BSE = eq(128083204, 'BSE', 'RELIANCE', 'RELIANCE INDUSTRIES');
export const SBIN = eq(779521, 'NSE', 'SBIN', 'STATE BANK OF INDIA');
export const TCS = eq(2953217, 'NSE', 'TCS', 'TATA CONSULTANCY SERVICES');
export const INFY = eq(408065, 'NSE', 'INFY', 'INFOSYS');
export const ADANIENT = eq(6401, 'NSE', 'ADANIENT', 'ADANI ENTERPRISES');
export const NEWCO = eq(999001, 'NSE', 'NEWCO', 'NEWLY LISTED CO'); // no history (Review Focus 1)
export const RELIANCE_OCT_FUT = fut(13368834, 'NFO', 'RELIANCE26OCTFUT', 'RELIANCE', 500);
export const NIFTY_OCT_FUT = fut(13238530, 'NFO', 'NIFTY26OCTFUT', 'NIFTY', 75);
export const BANKNIFTY_OCT_FUT = fut(13237250, 'NFO', 'BANKNIFTY26OCTFUT', 'BANKNIFTY', 35);
export const SENSEX_OCT_FUT = fut(285769221, 'BFO', 'SENSEX26OCTFUT', 'SENSEX', 20);
/** Out of scope — must never render anywhere (NFR-15). */
export const NIFTY_OPTION: Instrument = {
  token: 12345678, exchange: 'NFO', segment: 'NFO-OPT', tag: 'NFO-OPT', tradingsymbol: 'NIFTY26OCT22500CE', name: 'NIFTY',
  displayName: 'NIFTY OCT 22500 CE', instrumentType: 'CE', expiryMs: OCT_2026_EXPIRY, tickSizePaise: 5, lotSize: 75,
  isIndex: false, isFuture: false, underlying: 'NIFTY',
};

export const ALL_IN_SCOPE: readonly Instrument[] = [
  NIFTY50, NIFTYBANK, SENSEX, BANKEX, RELIANCE, RELIANCE_BSE, SBIN, TCS, INFY, ADANIENT, NEWCO,
  RELIANCE_OCT_FUT, NIFTY_OCT_FUT, BANKNIFTY_OCT_FUT, SENSEX_OCT_FUT,
];
