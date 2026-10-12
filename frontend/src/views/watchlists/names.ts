/** FR-WATCH-002 naming rules, mirroring Go's validateName / nextDefaultName (internal/service/watchlists.go). */
export const WATCHLIST_NAME_MAX = 40;

/** Go counts runes (utf8.RuneCountInString); spreading a string counts code points, which is the same thing. */
const runeCount = (s: string): number => [...s].length;
const fold = (s: string): string => s.trim().toLowerCase();

export function nextWatchlistName(existing: readonly string[]): string {
  const used = new Set(existing.map(fold));
  for (let n = 1; ; n++) {
    const candidate = `Watchlist ${n}`;
    if (!used.has(candidate.toLowerCase())) return candidate;
  }
}

export function validateWatchlistName(name: string, others: readonly string[]): string | null {
  const t = name.trim();
  if (t.length === 0) return 'Enter a name';
  if (runeCount(t) > WATCHLIST_NAME_MAX) return `Use ${WATCHLIST_NAME_MAX} characters or fewer`;
  if (others.some((o) => fold(o) === t.toLowerCase())) return 'A watchlist with this name exists';
  return null;
}
