const resets: (() => void)[] = [];

export function registerReset(fn: () => void): void {
  resets.push(fn);
}

/** Test helper: return every registered store to its initial state. */
export function resetAllStores(): void {
  for (const reset of resets) reset();
}
