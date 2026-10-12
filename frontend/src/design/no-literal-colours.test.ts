import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join, relative, sep } from 'node:path';
import { expect, it } from 'vitest';

const SRC = join(process.cwd(), 'src');
const LITERAL = /(?<![\w-])#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})(?![\w-])|\b(?:rgba?|hsla?)\(/;

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    return statSync(p).isDirectory() ? walk(p) : [p];
  });
}

it('has no colour literals outside src/design (ARCHITECTURE §8.6, §10.5)', () => {
  const offenders = walk(SRC)
    .filter((f) => /\.(tsx?|css)$/.test(f))
    .filter((f) => !relative(SRC, f).startsWith(`design${sep}`))
    .filter((f) => !/\.test\.tsx?$/.test(f))
    .filter((f) => LITERAL.test(readFileSync(f, 'utf8')));
  expect(offenders.map((f) => relative(SRC, f))).toEqual([]);
});
