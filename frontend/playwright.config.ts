import { defineConfig } from '@playwright/test';

const port = Number(process.env.E2E_PORT ?? 5174);

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    viewport: { width: 1280, height: 720 },
    trace: 'retain-on-failure',
  },
  // WebKit is the engine Wails uses on Linux (WebKitGTK); chromium catches engine-specific layout bugs.
  projects: [
    { name: 'webkit', use: { browserName: 'webkit' } },
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: `npm run dev:mock -- --host 127.0.0.1 --port ${port} --strictPort`,
    url: `http://127.0.0.1:${port}`,
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
