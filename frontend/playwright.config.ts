import { defineConfig } from '@playwright/test';

// E2E_PORT lets parallel worktrees use different ports. Never 5180 (the user's other app owns it).
const PORT = Number(process.env.E2E_PORT ?? 5174);
if (!Number.isInteger(PORT) || PORT === 5180) throw new Error(`E2E_PORT must be an integer other than 5180 (got ${process.env.E2E_PORT})`);
const BASE_URL = `http://127.0.0.1:${PORT}`;

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  use: {
    baseURL: BASE_URL,
    viewport: { width: 1280, height: 720 },
    trace: 'retain-on-failure',
  },
  // WebKit is the engine Wails uses on Linux (WebKitGTK); chromium catches engine-specific layout bugs.
  projects: [
    { name: 'webkit', use: { browserName: 'webkit' } },
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: `npm run dev:mock -- --host 127.0.0.1 --port ${PORT} --strictPort`,
    url: BASE_URL,
    // Opt-in only: reusing whatever already listens on PORT could drive someone else's server.
    reuseExistingServer: !process.env.CI && process.env.E2E_REUSE === '1',
    timeout: 60_000,
  },
});
