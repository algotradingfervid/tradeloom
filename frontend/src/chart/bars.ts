import type { Bar } from './types';

export function dedupeSorted(bars: readonly Bar[]): Bar[] {
  const byTime = new Map<number, Bar>();
  for (const b of bars) byTime.set(b.time, b);
  return [...byTime.values()].sort((a, b) => a.time - b.time);
}

export function mergeBars(existing: readonly Bar[], incoming: readonly Bar[]): Bar[] {
  return dedupeSorted([...existing, ...incoming]);
}
