/** Client-side, case-insensitive "contains" filter over the given fields (FR-SHELL-014). */
export function filterRows<T>(rows: T[], query: string, fields: (row: T) => (string | undefined)[]): T[] {
  const q = query.trim().toLowerCase();
  if (!q) return rows;
  return rows.filter((row) => fields(row).some((f) => f?.toLowerCase().includes(q)));
}
