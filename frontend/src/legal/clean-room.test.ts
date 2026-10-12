import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { ARTICLES } from '../help/articles';
import { LEGAL_PAGES } from './content';

// FR-ACCT-015 AC3 + global "own copy only": headings and labels transcribed from the research screenshots
// (docs/research/FEATURES.md SHELL/ORD/ACCT blocks) must not appear in our text.
const FORBIDDEN = [
  'streak', 'terms & conditions', 'important regulatory clarification', 'privacy policy', 'notification-based trading',
  'equity trader', 'options trader', 'scalper mode', 'advance create', 'still need help', 'search for help topics',
  'browse help topics', 'remember f&o order quantity', 'push notifications', 'orderbook', 'no positions found',
  'fill all the conditions before running scan', 'strategy saved successfully', 'copy the scanner in order to set alerts',
];

const corpus = [
  readFileSync(join(process.cwd(), 'src/i18n/copy.ts'), 'utf8'),
  ...Object.values(LEGAL_PAGES).flatMap((p) => [p.title, ...p.paragraphs]),
  ...ARTICLES.flatMap((a) => [a.title, a.body]),
].join('\n').toLowerCase();

describe('clean-room text check', () => {
  it.each(FORBIDDEN)('does not contain "%s"', (phrase) => {
    expect(corpus.includes(phrase)).toBe(false);
  });
});
