// @vitest-environment node
import { mkdtempSync, readdirSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { build } from 'vite';
import { describe, expect, it } from 'vitest';

// Builds into a throwaway directory (no dev server, no port) and scans the emitted JavaScript (Ruling 83-F M7).
async function bundleText(mode: string): Promise<string> {
  const outDir = mkdtempSync(join(tmpdir(), 'tradeloom-build-'));
  const saved = process.env.VITE_BACKEND;
  delete process.env.VITE_BACKEND;
  try {
    await build({
      root: resolve(__dirname, '../..'),
      mode,
      logLevel: 'silent',
      build: { outDir, emptyOutDir: true },
    });
    const assets = join(outDir, 'assets');
    return readdirSync(assets).filter((f) => f.endsWith('.js')).map((f) => readFileSync(join(assets, f), 'utf8')).join('\n');
  } finally {
    if (saved !== undefined) process.env.VITE_BACKEND = saved;
    rmSync(outDir, { recursive: true, force: true });
  }
}

describe('production bundle', () => {
  it('ships neither the mock backend nor its fixtures; the mock-mode bundle does (control)', async () => {
    const prod = await bundleText('production');
    expect(prod).not.toContain('__tradeloomMock');
    expect(prod).not.toContain('kiteapikey9f2c');
    const mock = await bundleText('mock');
    expect(mock).toContain('__tradeloomMock');
  }, 120_000);
});
