import { formatPrice } from '../i18n/format';

export function precisionForTick(tick: number): number {
  if (!(tick > 0)) return 2;
  const decimals = (String(tick).split('.')[1] ?? '').length;
  return Math.max(2, decimals);
}

export function chartPriceFormatter(decimals: number): (p: number) => string {
  return (p) => formatPrice(p, decimals);
}
