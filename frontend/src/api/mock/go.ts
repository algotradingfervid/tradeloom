/** Small ports of Go standard-library formatting the mock needs to reproduce Go's refusal texts byte for byte. */

/** Go `len(s)`: the UTF-8 byte length. */
export const byteLen = (s: string): number => new TextEncoder().encode(s).length;

// Go unicode.IsSpace (Latin-1 spaces plus the White_Space property).
const GO_SPACE = '\\t\\n\\v\\f\\r \\u0085\\u00a0\\u1680\\u2000-\\u200a\\u2028\\u2029\\u202f\\u205f\\u3000';
const TRIM_RE = new RegExp(`^[${GO_SPACE}]+|[${GO_SPACE}]+$`, 'gu');

/** Go `strings.TrimSpace`. */
export const goTrimSpace = (s: string): string => s.replace(TRIM_RE, '');

// Go unicode.IsPrint: letters, marks, numbers, punctuation, symbols and the ASCII space.
const PRINTABLE = /^[\p{L}\p{M}\p{N}\p{P}\p{S} ]$/u;
const NAMED_ESCAPES: Record<string, string> = { '\x07': '\\a', '\b': '\\b', '\f': '\\f', '\n': '\\n', '\r': '\\r', '\t': '\\t', '\v': '\\v' };

/** Go `strconv.Quote` (`%q` of a string). */
export function goQuote(s: string): string {
  let out = '"';
  for (const ch of s) {
    const cp = ch.codePointAt(0)!;
    if (ch === '"' || ch === '\\') out += `\\${ch}`;
    else if (PRINTABLE.test(ch)) out += ch;
    else if (NAMED_ESCAPES[ch]) out += NAMED_ESCAPES[ch];
    else if (cp < 0x20 || cp === 0x7f) out += `\\x${cp.toString(16).padStart(2, '0')}`;
    else if (cp < 0x10000) out += `\\u${cp.toString(16).padStart(4, '0')}`;
    else out += `\\U${cp.toString(16).padStart(8, '0')}`;
  }
  return `${out}"`;
}

/** Go `fmt` `%v` of a []string. */
export const goStringSlice = (xs: readonly string[]): string => `[${xs.join(' ')}]`;

/** Go `json.Marshal` of a string: like JSON.stringify, but HTML-safe (<, >, &) and with U+2028/U+2029 escaped. */
export const goMarshalString = (s: string): string =>
  JSON.stringify(s).replace(/[<>&\u2028\u2029]/g, (c) => `\\u${c.charCodeAt(0).toString(16).padStart(4, '0')}`);
