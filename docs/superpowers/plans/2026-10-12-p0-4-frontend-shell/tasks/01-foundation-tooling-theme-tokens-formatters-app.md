> **P0-4 Frontend Shell, Account & Data UI Implementation Plan — Task 1.** Read [`../MANIFEST.md`](../MANIFEST.md) first: its Global Constraints,
> Review Focus and Consumes/Produces apply to this task. Other tasks live beside this file in `tasks/`.

<!-- task body (verbatim from the plan) -->
### Task 1: Foundation — tooling, theme tokens, formatters, app layout shell (reused by P0-5)

**Files:**
- Modify: `frontend/package.json` (dependencies, scripts)
- Overwrite: `frontend/vite.config.ts`, `frontend/src/main.tsx`, `frontend/src/vite-env.d.ts`
- Create: `frontend/eslint.config.js`, `frontend/.env.mock`, `frontend/playwright.config.ts`, `frontend/e2e/shell.spec.ts`
- Create: `frontend/src/test/setup.ts`, `frontend/src/test/matchMedia.ts`
- Create: `frontend/src/design/tokens.ts`, `frontend/src/design/contrast.ts`, `frontend/src/design/tokens.css`, `frontend/src/design/base.css`
- Create: `frontend/src/i18n/format.ts`, `frontend/src/i18n/copy.ts`
- Create: `frontend/src/app/features.ts`, `frontend/src/app/theme.tsx`, `frontend/src/app/AppShell.tsx`, `frontend/src/app/TopBar.tsx`, `frontend/src/app/Sidebar.tsx`, `frontend/src/app/HomeLayout.tsx`, `frontend/src/app/ShellLayout.tsx`, `frontend/src/app/RootLayout.tsx`, `frontend/src/app/routes.tsx`, `frontend/src/app/router.ts`, `frontend/src/app/App.tsx`, `frontend/src/app/shell.css`
- Create: `frontend/src/components/Icon.tsx`, `frontend/src/views/home/HomeView.tsx`
- Test: `frontend/src/i18n/format.test.ts`, `frontend/src/design/tokens.test.ts`, `frontend/src/design/no-literal-colours.test.ts`, `frontend/src/app/features.test.ts`, `frontend/src/app/theme.test.tsx`, `frontend/src/app/Sidebar.test.tsx`, `frontend/src/app/AppShell.test.tsx`

**Interfaces:**
- Consumes: P0-1 `frontend/` scaffold (see "Consumes (from P0-1)").
- Produces (**P0-5 reuses all of these; names and props are frozen after this task**):
  - `src/design/tokens.ts`: `type ThemeName = 'light' | 'dark'`; `interface ColourTokens { bg; surface; surface2; border; text; textMuted; accent; onAccent; up; down; warn; flat; focus; overlay; shadow: string }`; `const colourTokens: Record<ThemeName, ColourTokens>`; `cssVarName(key: keyof ColourTokens): string`.
  - `src/app/theme.tsx`: `applyTheme(name: ThemeName, root?: HTMLElement): void`; `ThemeProvider({ mode: ThemeName; followSystem: boolean; children: ReactNode })`; `useTheme(): { theme: ThemeName; tokens: ColourTokens }`.
  - `src/i18n/format.ts`: `DASH`, `formatNumber(v, digits = 2)`, `formatPrice(v, tickDecimals = 2)`, `formatMoney(v, digits = 2)`, `formatSignedMoney(v, digits = 2)`, `formatSigned(v, digits = 2)`, `formatPercent(v, digits = 2)`, `formatChange(abs, pct)`, `formatCompact(v)`, `formatDate(ms)`, `formatDateLong(ms)`, `formatDateTimeLong(ms)`, `formatDateTimeCompact(ms)`, `formatTime(ms, withSeconds = false)`, `formatExpiry(expiresAt, now)`, `istDayKey(ms): string` (`YYYY-MM-DD` in IST), `toneOf(v): 'up' | 'down' | 'flat'`. Every `v` parameter is `number | null | undefined`; every result is a `string`.
  - `src/app/features.ts`: `type Phase = 0|1|2|3|4|5|6`; `BUILD_PHASE: Phase`; `type NavId`; `interface NavEntry { id: NavId; label: string; path: string; icon: IconName; phase: Phase; shortcutId: string }`; `NAV_ENTRIES`; `visibleNavEntries(phase?: Phase): NavEntry[]`; `type Feature = 'notificationsBell' | 'orderButtons' | 'liveStrategyCard' | 'profileToggles'`; `isEnabled(f: Feature, phase?: Phase): boolean`.
  - `src/app/AppShell.tsx`: `AppShell({ topBar: ReactNode; sidebar: ReactNode; banner?: ReactNode; children: ReactNode })`.
  - `src/app/TopBar.tsx`: `TopBar({ ticker?: ReactNode; status?: ReactNode; links?: ReactNode; profile?: ReactNode })` — left to right: ticker, status, spacer, links, profile (FR-SHELL-001).
  - `src/app/Sidebar.tsx`: `Sidebar({ entries: readonly NavEntry[]; collapsed: boolean; onToggleCollapsed: () => void })`.
  - `src/app/HomeLayout.tsx`: `HomeLayout({ topRow?: ReactNode; main: ReactNode; below?: ReactNode; right: ReactNode })` — left column (~2/3) and right column (~1/3), each its own scroll container (`data-testid="home-left"` / `"home-right"`).
  - `src/app/ShellLayout.tsx` (route element composing the above; P0-5 sets `ticker`), `src/app/routes.tsx` (`shellChildren`, `rootChildren`, `buildRoutes()`), `src/app/router.ts` (`type AppRouter`, `createAppRouter(initialEntries?: string[])`).
  - `src/components/Icon.tsx`: `type IconName`; `Icon({ name: IconName; size?: number; className?: string })`.

- [ ] **Step 1: Install pinned dependencies, scripts and test configuration**

```bash
cd frontend
npm install -E react@19.3.0 react-dom@19.3.0 react-router-dom@7.18.4 zustand@5.0.15
npm install -E -D vite@8.3.4 @vitejs/plugin-react@6.1.2 vitest@5.0.4 jsdom@30.1.2 \
  @testing-library/react@16.3.3 @testing-library/dom@10.4.2 @testing-library/user-event@14.6.7 \
  @testing-library/jest-dom@7.0.1 @playwright/test@1.64.0 @axe-core/playwright@4.13.0 \
  @types/react@19.3.0 @types/react-dom@19.3.0 @types/node@22.20.5
npm install -E -D eslint@10.12.0 @eslint/js@10.0.1 typescript-eslint@8.71.1 eslint-plugin-react-hooks@7.1.1 globals@17.13.0
# P0-1 already has dev, build, typecheck and test; this adds lint, e2e and dev:mock
npm pkg set scripts.lint="eslint src e2e" scripts.e2e="playwright test" scripts.dev:mock="vite --mode mock"
npx playwright install --with-deps chromium webkit
# remove the Wails template demo files (they contain colour literals and are replaced by this plan)
rm -f src/App.tsx src/App.css src/style.css && rm -rf src/assets
```

Overwrite `frontend/vite.config.ts`:

```ts
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    restoreMocks: true,
  },
});
```

Create `frontend/eslint.config.js` (ESLint 10 flat config; `wailsjs/` is generated and not linted):

```js
import js from '@eslint/js';
import { defineConfig } from 'eslint/config';
import reactHooks from 'eslint-plugin-react-hooks';
import globals from 'globals';
import tseslint from 'typescript-eslint';

export default defineConfig([
  { ignores: ['dist/**', 'wailsjs/**'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, tseslint.configs.recommended],
    plugins: { 'react-hooks': reactHooks },
    languageOptions: { globals: { ...globals.browser, ...globals.node } },
    rules: {
      'react-hooks/rules-of-hooks': 'error',
      'react-hooks/exhaustive-deps': 'warn',
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_', varsIgnorePattern: '^_' }],
    },
  },
]);
```

Create `frontend/.env.mock`:

```
VITE_BACKEND=mock
```

Overwrite `frontend/src/vite-env.d.ts`:

```ts
/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_BACKEND?: 'mock' | 'wails';
  readonly VITE_PHASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
```

Create `frontend/src/test/matchMedia.ts`:

```ts
type Listener = (e: MediaQueryListEvent) => void;

let systemDark = false;
const listeners = new Set<Listener>();

export function setSystemDark(value: boolean): void {
  systemDark = value;
  for (const l of [...listeners]) l({ matches: value, media: '(prefers-color-scheme: dark)' } as MediaQueryListEvent);
}

export function resetMatchMedia(): void {
  systemDark = false;
  listeners.clear();
}

export function installMatchMedia(): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string) => ({
      get matches() {
        return query.includes('dark') ? systemDark : false;
      },
      media: query,
      onchange: null,
      addEventListener: (_type: string, l: Listener) => listeners.add(l),
      removeEventListener: (_type: string, l: Listener) => listeners.delete(l),
      addListener: (l: Listener) => listeners.add(l),
      removeListener: (l: Listener) => listeners.delete(l),
      dispatchEvent: () => true,
    }),
  });
}
```

Create `frontend/src/test/setup.ts`:

```ts
import '@testing-library/jest-dom/vitest';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';
import { installMatchMedia, resetMatchMedia } from './matchMedia';

installMatchMedia();

afterEach(() => {
  cleanup();
  resetMatchMedia();
});
```

Create `frontend/playwright.config.ts`:

```ts
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  use: {
    baseURL: 'http://127.0.0.1:5174',
    viewport: { width: 1280, height: 720 },
    trace: 'retain-on-failure',
  },
  // WebKit is the engine Wails uses on Linux (WebKitGTK); chromium catches engine-specific layout bugs.
  projects: [
    { name: 'webkit', use: { browserName: 'webkit' } },
    { name: 'chromium', use: { browserName: 'chromium' } },
  ],
  webServer: {
    command: 'npm run dev:mock -- --host 127.0.0.1 --port 5174 --strictPort',
    url: 'http://127.0.0.1:5174',
    reuseExistingServer: !process.env.CI,
    timeout: 60_000,
  },
});
```

If P0-1's `tsconfig.json` `include` lacks them, add `"e2e"` and `"playwright.config.ts"` to it.

- [ ] **Step 2: Write the failing tests for tokens, formatters and features**

Create `frontend/src/i18n/format.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import {
  DASH, formatChange, formatCompact, formatDate, formatDateLong, formatDateTimeCompact, formatDateTimeLong,
  formatExpiry, formatMoney, formatNumber, formatPercent, formatPrice, formatSigned, formatSignedMoney, formatTime,
  istDayKey, toneOf,
} from './format';

// 09 Oct 2026 15:15:32 IST
const T = Date.UTC(2026, 9, 9, 9, 45, 32);

describe('FR-SHELL-016 numbers', () => {
  it('AC1 abbreviates OI to crore and lakh', () => {
    expect(formatCompact(13_200_000)).toBe('1.32Cr');
    expect(formatCompact(254_000)).toBe('2.54L');
    expect(formatCompact(99_999)).toBe('99,999');
  });
  it('AC2 formats money with ₹ and Indian grouping', () => {
    expect(formatMoney(1234567.8)).toBe('₹12,34,567.80');
  });
  it('formats prices at tick precision', () => {
    expect(formatPrice(22520.45)).toBe('22,520.45');
    expect(formatPrice(1.5, 4)).toBe('1.5000');
  });
  it('formats signed change with percent in parentheses (FR-SHELL-004 AC2 style)', () => {
    expect(formatChange(100, 0.4444)).toBe('+100.00 (+0.44%)');
    expect(formatSigned(-12.3)).toBe('-12.30');
    expect(formatPercent(10)).toBe('+10.00%');
  });
  it('classifies tone', () => {
    expect(toneOf(1)).toBe('up');
    expect(toneOf(-0.01)).toBe('down');
    expect(toneOf(-0.001)).toBe('flat'); // displays as 0.00, so no red
    expect(toneOf(0)).toBe('flat');
    expect(toneOf(null)).toBe('flat');
  });
});

describe('FR-SHELL-016 dates (IST regardless of OS zone)', () => {
  it('renders the long and compact styles', () => {
    expect(formatDateTimeLong(T)).toBe('09 Oct 2026, 15:15');
    expect(formatDateTimeCompact(T)).toBe('09-10-2026 15:15:32');
    expect(formatDate(T)).toBe('09-10-2026');
    expect(formatDateLong(T)).toBe('09 Oct 2026');
    expect(formatTime(T)).toBe('15:15');
    expect(formatTime(T, true)).toBe('15:15:32');
  });
  it('uses our own month table (ICU en-IN would print "Sept")', () => {
    expect(formatDateLong(Date.UTC(2026, 8, 9, 6, 0))).toBe('09 Sep 2026');
  });
  it('rolls the date at IST midnight and never prints 24:00', () => {
    expect(formatDateTimeCompact(Date.UTC(2026, 9, 9, 18, 30, 0))).toBe('10-10-2026 00:00:00');
    expect(istDayKey(Date.UTC(2026, 9, 9, 18, 30, 0))).toBe('2026-10-10');
  });
  it('describes session expiry relative to the app clock (FR-ACCT-001 AC3)', () => {
    const loginAt0902 = Date.UTC(2026, 9, 9, 3, 32);
    const nextSix = Date.UTC(2026, 9, 10, 0, 30);
    expect(formatExpiry(nextSix, loginAt0902)).toBe('06:00 tomorrow');
    expect(formatExpiry(nextSix, Date.UTC(2026, 9, 9, 20, 0))).toBe('06:00 today');
    expect(formatExpiry(Date.UTC(2026, 9, 12, 0, 30), loginAt0902)).toBe('12 Oct 2026, 06:00');
  });
});

describe('review focus: very large, negative and non-finite values', () => {
  it('keeps Indian grouping and one leading sign for large and negative values', () => {
    expect(formatMoney(-1234567890.5)).toBe('-₹1,23,45,67,890.50');
    expect(formatNumber(1e15)).toBe('1,00,00,00,00,00,00,000.00');
    expect(formatSignedMoney(-45.5)).toBe('-₹45.50');
    expect(formatSignedMoney(50)).toBe('+₹50.00');
    expect(formatCompact(-13_200_000)).toBe('-1.32Cr');
    expect(formatPercent(-2.5)).toBe('-2.50%');
  });
  it('rolls lakh into crore and units into lakh at rounding boundaries', () => {
    expect(formatCompact(9_999_999.999)).toBe('1.00Cr');
    expect(formatCompact(99_999.996)).toBe('1.00L');
  });
  it('never renders negative zero', () => {
    expect(formatNumber(-0.001)).toBe('0.00');
    expect(formatSignedMoney(-0.004)).toBe('+₹0.00');
    expect(formatMoney(-0)).toBe('₹0.00');
  });
  it('renders an em dash for missing or non-finite values, never 0.00', () => {
    for (const v of [NaN, Infinity, -Infinity, null, undefined]) {
      expect(formatNumber(v)).toBe(DASH);
      expect(formatMoney(v)).toBe(DASH);
      expect(formatSignedMoney(v)).toBe(DASH);
      expect(formatPercent(v)).toBe(DASH);
      expect(formatCompact(v)).toBe(DASH);
      expect(formatChange(v, 1)).toBe(DASH);
    }
    expect(formatDate(0)).toBe(DASH);
    expect(formatDateTimeLong(NaN)).toBe(DASH);
  });
});
```

Create `frontend/src/design/tokens.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { colourTokens, cssVarName } from './tokens';
import { contrastRatio } from './contrast';

const FOREGROUNDS = ['text', 'textMuted', 'accent', 'up', 'down', 'warn', 'flat'] as const;
const BACKGROUNDS = ['bg', 'surface', 'surface2'] as const;
const PAIRS = FOREGROUNDS.flatMap((f) => BACKGROUNDS.map((b) => [f, b] as const));

describe.each(['light', 'dark'] as const)('FR-SHELL-011 AC3 — %s theme contrast', (name) => {
  const t = colourTokens[name];
  it.each(PAIRS)('%s on %s ≥ 4.5:1', (fg, bg) => {
    expect(contrastRatio(t[fg], t[bg])).toBeGreaterThanOrEqual(4.5);
  });
  it('onAccent on accent ≥ 4.5:1', () => {
    expect(contrastRatio(t.onAccent, t.accent)).toBeGreaterThanOrEqual(4.5);
  });
});

describe('token plumbing', () => {
  it('both themes define the same keys', () => {
    expect(Object.keys(colourTokens.dark).sort()).toEqual(Object.keys(colourTokens.light).sort());
  });
  it('maps camelCase keys to kebab-case CSS variables', () => {
    expect(cssVarName('surface2')).toBe('--surface-2');
    expect(cssVarName('textMuted')).toBe('--text-muted');
  });
});
```

Create `frontend/src/design/no-literal-colours.test.ts`:

```ts
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
```

Create `frontend/src/app/features.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { BUILD_PHASE, NAV_ENTRIES, isEnabled, visibleNavEntries } from './features';

describe('phase-gated features (ROADMAP §0)', () => {
  it('defaults to phase 0', () => {
    expect(BUILD_PHASE).toBe(0);
  });
  it('P0 sidebar shows only Home', () => {
    expect(visibleNavEntries(0).map((e) => e.label)).toEqual(['Home']);
  });
  it('FR-SHELL-002 AC1 (run in P6): eight entries in order', () => {
    expect(visibleNavEntries(6).map((e) => e.label)).toEqual([
      'Home', 'Scalper', 'Strategies', 'Equity Builder', 'Live Strategies', 'Scanners', 'New Scanner', 'Scanner Alerts',
    ]);
  });
  it('binds Alt+1…Alt+8 to the entries in order', () => {
    expect(NAV_ENTRIES.map((e) => e.shortcutId)).toEqual(['nav.1', 'nav.2', 'nav.3', 'nav.4', 'nav.5', 'nav.6', 'nav.7', 'nav.8']);
  });
  it('no order buttons and no bell in P0 builds', () => {
    expect(isEnabled('orderButtons', 0)).toBe(false);
    expect(isEnabled('notificationsBell', 0)).toBe(false);
    expect(isEnabled('orderButtons', 4)).toBe(true);
    expect(isEnabled('notificationsBell', 3)).toBe(true);
  });
});
```

- [ ] **Step 3: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/i18n src/design src/app/features.test.ts`
Expected: FAIL — `Failed to resolve import "./format"` (and `./tokens`, `./contrast`, `./features`); the literal-colour test passes (no source files yet).

- [ ] **Step 4: Implement tokens, contrast, formatters, copy and features**

Create `frontend/src/design/tokens.ts`:

```ts
export type ThemeName = 'light' | 'dark';

export interface ColourTokens {
  bg: string;
  surface: string;
  surface2: string;
  border: string;
  text: string;
  textMuted: string;
  accent: string;
  onAccent: string;
  up: string;
  down: string;
  warn: string;
  flat: string;
  focus: string;
  overlay: string;
  shadow: string;
}

// Values checked for WCAG AA in tokens.test.ts. The only place colour literals are allowed (ARCHITECTURE §8.6).
export const colourTokens: Record<ThemeName, ColourTokens> = {
  light: {
    bg: '#ffffff', surface: '#f6f7f9', surface2: '#eceef2', border: '#d0d5dd',
    text: '#1a1d23', textMuted: '#525a67', accent: '#1f5fbf', onAccent: '#ffffff',
    up: '#127031', down: '#b3261e', warn: '#8a5a00', flat: '#5f6672', focus: '#1f5fbf',
    overlay: 'rgba(16, 18, 22, 0.45)', shadow: '0 4px 16px rgba(16, 18, 22, 0.16)',
  },
  dark: {
    bg: '#0f1115', surface: '#171a21', surface2: '#1f232c', border: '#343a46',
    text: '#e6e8ec', textMuted: '#a3abb9', accent: '#6ea8ff', onAccent: '#0f1115',
    up: '#4cc38a', down: '#ff7b72', warn: '#f0b429', flat: '#9aa3b2', focus: '#6ea8ff',
    overlay: 'rgba(0, 0, 0, 0.6)', shadow: '0 4px 16px rgba(0, 0, 0, 0.5)',
  },
};

export function cssVarName(key: keyof ColourTokens): string {
  return '--' + key.replace(/[A-Z0-9]/g, (c) => '-' + c.toLowerCase());
}
```

Create `frontend/src/design/contrast.ts`:

```ts
function channel(c: number): number {
  const s = c / 255;
  return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
}

export function luminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) throw new Error(`not a #rrggbb colour: ${hex}`);
  const n = parseInt(m[1], 16);
  return 0.2126 * channel((n >> 16) & 255) + 0.7152 * channel((n >> 8) & 255) + 0.0722 * channel(n & 255);
}

export function contrastRatio(a: string, b: string): number {
  const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
  return (hi + 0.05) / (lo + 0.05);
}
```

Create `frontend/src/design/tokens.css`:

```css
:root {
  --space-1: 4px; --space-2: 8px; --space-3: 12px; --space-4: 16px; --space-5: 20px; --space-6: 24px; --space-8: 32px;
  --radius-1: 4px; --radius-2: 8px; --radius-pill: 999px;
  --font-ui: "Inter", "Noto Sans", "DejaVu Sans", system-ui, sans-serif;
  --font-mono: "JetBrains Mono", "DejaVu Sans Mono", ui-monospace, monospace;
  --fs-xs: 12px; --fs-sm: 13px; --fs-md: 14px; --fs-lg: 16px; --fs-xl: 20px; --fs-2xl: 24px;
  --topbar-h: 48px; --sidebar-w: 208px; --sidebar-w-collapsed: 56px;
  --motion-fast: 150ms; --motion-colour: 300ms;
  --z-menu: 20; --z-dialog: 40; --z-toast: 60; --z-tooltip: 80;
}
```

Create `frontend/src/design/base.css`:

```css
*, *::before, *::after { box-sizing: border-box; }
html, body, #root { height: 100%; margin: 0; }
body {
  font-family: var(--font-ui); font-size: var(--fs-md); line-height: 1.4;
  background: var(--bg); color: var(--text);
  transition: background-color var(--motion-colour), color var(--motion-colour);
}
h1, h2, h3 { margin: 0; font-weight: 600; }
h1 { font-size: var(--fs-xl); }
h2 { font-size: var(--fs-lg); }
h3 { font-size: var(--fs-md); }
p { margin: 0; }
a { color: var(--accent); }
button, input, select, textarea { font: inherit; color: inherit; }
:focus-visible { outline: 2px solid var(--focus); outline-offset: 2px; }
.visually-hidden {
  position: absolute !important; width: 1px; height: 1px; padding: 0; margin: -1px;
  overflow: hidden; clip: rect(0 0 0 0); white-space: nowrap; border: 0;
}
.muted { color: var(--text-muted); }
.num { font-variant-numeric: tabular-nums; text-align: right; white-space: nowrap; }
.tone-up { color: var(--up); }
.tone-down { color: var(--down); }
.tone-flat { color: var(--text-muted); }
.card { background: var(--surface); border: 1px solid var(--border); border-radius: var(--radius-2); padding: var(--space-4); }
.icon { flex: none; }
```

Create `frontend/src/i18n/format.ts`:

```ts
export const DASH = '—';

const TZ = 'Asia/Kolkata';
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const DAY_MS = 86_400_000;

type Num = number | null | undefined;

const numberFormats = new Map<number, Intl.NumberFormat>();
function grouping(digits: number): Intl.NumberFormat {
  let f = numberFormats.get(digits);
  if (!f) {
    f = new Intl.NumberFormat('en-IN', { minimumFractionDigits: digits, maximumFractionDigits: digits, useGrouping: true });
    numberFormats.set(digits, f);
  }
  return f;
}

function isNum(v: Num): v is number {
  return typeof v === 'number' && Number.isFinite(v);
}

/** Round to `digits` decimals and normalise -0 to 0 so "-0.00" can never be printed. */
function roundTo(v: number, digits: number): number {
  const r = Number(v.toFixed(digits));
  return r === 0 ? 0 : r;
}

function signed(v: number, digits: number, prefix: string): string {
  const r = roundTo(v, digits);
  return (r < 0 ? '-' : '+') + prefix + grouping(digits).format(Math.abs(r));
}

export function formatNumber(v: Num, digits = 2): string {
  return isNum(v) ? grouping(digits).format(roundTo(v, digits)) : DASH;
}

export function formatPrice(v: Num, tickDecimals = 2): string {
  return formatNumber(v, tickDecimals);
}

export function formatMoney(v: Num, digits = 2): string {
  if (!isNum(v)) return DASH;
  const r = roundTo(v, digits);
  return (r < 0 ? '-' : '') + '₹' + grouping(digits).format(Math.abs(r));
}

export function formatSignedMoney(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '₹') : DASH;
}

export function formatSigned(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '') : DASH;
}

export function formatPercent(v: Num, digits = 2): string {
  return isNum(v) ? signed(v, digits, '') + '%' : DASH;
}

export function formatChange(abs: Num, pct: Num): string {
  if (!isNum(abs) || !isNum(pct)) return DASH;
  return `${formatSigned(abs)} (${formatPercent(pct)})`;
}

export function formatCompact(v: Num): string {
  if (!isNum(v)) return DASH;
  const a = Math.abs(v);
  const sign = v < 0 ? '-' : '';
  if (a >= 1e7 || roundTo(a / 1e5, 2) >= 100) return sign + grouping(2).format(roundTo(a / 1e7, 2)) + 'Cr';
  if (a >= 1e5 || Math.round(a) >= 1e5) return sign + grouping(2).format(roundTo(a / 1e5, 2)) + 'L';
  return formatNumber(v, 0);
}

export function toneOf(v: Num): 'up' | 'down' | 'flat' {
  if (!isNum(v)) return 'flat';
  const r = roundTo(v, 2);
  return r > 0 ? 'up' : r < 0 ? 'down' : 'flat';
}

const dateTimeParts = new Intl.DateTimeFormat('en-IN', {
  timeZone: TZ, year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23',
});

interface IstParts { y: number; m: number; d: number; hh: string; mm: string; ss: string }

function istParts(ms: number): IstParts {
  const p: Record<string, string> = {};
  for (const part of dateTimeParts.formatToParts(new Date(ms))) p[part.type] = part.value;
  return { y: Number(p.year), m: Number(p.month), d: Number(p.day), hh: p.hour, mm: p.minute, ss: p.second };
}

const pad2 = (n: number) => String(n).padStart(2, '0');
const validTime = (ms: Num): ms is number => isNum(ms) && ms > 0;

export function formatDate(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${pad2(p.d)}-${pad2(p.m)}-${p.y}`;
}

export function formatDateLong(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${pad2(p.d)} ${MONTHS[p.m - 1]} ${p.y}`;
}

export function formatDateTimeLong(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${formatDateLong(ms)}, ${p.hh}:${p.mm}`;
}

export function formatDateTimeCompact(ms: Num): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return `${formatDate(ms)} ${p.hh}:${p.mm}:${p.ss}`;
}

export function formatTime(ms: Num, withSeconds = false): string {
  if (!validTime(ms)) return DASH;
  const p = istParts(ms);
  return withSeconds ? `${p.hh}:${p.mm}:${p.ss}` : `${p.hh}:${p.mm}`;
}

export function istDayKey(ms: number): string {
  const p = istParts(ms);
  return `${p.y}-${pad2(p.m)}-${pad2(p.d)}`;
}

function istDayIndex(ms: number): number {
  const p = istParts(ms);
  return Date.UTC(p.y, p.m - 1, p.d) / DAY_MS;
}

/** "06:00 today", "06:00 tomorrow", otherwise the long style. `now` is app-clock time. */
export function formatExpiry(expiresAt: Num, now: number): string {
  if (!validTime(expiresAt)) return DASH;
  const delta = istDayIndex(expiresAt) - istDayIndex(now);
  const time = formatTime(expiresAt);
  if (delta === 0) return `${time} today`;
  if (delta === 1) return `${time} tomorrow`;
  return formatDateTimeLong(expiresAt);
}
```

Create `frontend/src/i18n/copy.ts` (later tasks append further `export const …Copy` blocks to this file):

```ts
// All UI strings (ARCHITECTURE §8.1). Our own wording; never reuse third-party product text.

export const navCopy = {
  home: 'Home',
  scalper: 'Scalper',
  strategies: 'Strategies',
  builder: 'Equity Builder',
  live: 'Live Strategies',
  scanners: 'Scanners',
  newScanner: 'New Scanner',
  scannerAlerts: 'Scanner Alerts',
} as const;

export const shellCopy = {
  sidebarLabel: 'Main navigation',
  collapseSidebar: 'Collapse sidebar',
  expandSidebar: 'Expand sidebar',
  quickLinks: 'Quick links',
} as const;

export const homeCopy = {
  title: 'Home',
  rightColumn: 'Watchlists and widgets',
  chartArea: 'Main chart',
  watchlistArea: 'Watchlists',
} as const;
```

Create `frontend/src/components/Icon.tsx`:

```tsx
import type { ReactNode } from 'react';

const PATHS = {
  home: <path d="M3 11l9-7 9 7v9a1 1 0 0 1-1 1h-5v-6H9v6H4a1 1 0 0 1-1-1z" />,
  scalper: <path d="M13 2L4 14h7l-1 8 9-12h-7z" />,
  strategies: (<><rect x="3" y="3" width="7" height="7" /><rect x="14" y="3" width="7" height="7" /><rect x="3" y="14" width="7" height="7" /><rect x="14" y="14" width="7" height="7" /></>),
  builder: <path d="M4 20h4l10-10-4-4L4 16zM14 6l4 4" />,
  live: (<><circle cx="12" cy="12" r="3" /><path d="M5 5a10 10 0 0 0 0 14M19 5a10 10 0 0 1 0 14" /></>),
  scanners: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" /></>),
  newScanner: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5M11 8v6M8 11h6" /></>),
  alerts: <path d="M6 16v-5a6 6 0 0 1 12 0v5l2 2H4zM10 20a2 2 0 0 0 4 0" />,
  orders: (<><rect x="5" y="3" width="14" height="18" rx="1" /><path d="M8 8h8M8 12h8M8 16h5" /></>),
  positions: <path d="M3 17l6-6 4 4 8-8M15 7h6v6" />,
  holdings: (<><rect x="3" y="7" width="18" height="13" rx="1" /><path d="M8 7V4h8v3" /></>),
  user: (<><circle cx="12" cy="8" r="4" /><path d="M4 21a8 8 0 0 1 16 0" /></>),
  settings: (<><circle cx="12" cy="12" r="3" /><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9L7 7M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1" /></>),
  help: (<><circle cx="12" cy="12" r="9" /><path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.7M12 17h.01" /></>),
  info: (<><circle cx="12" cy="12" r="9" /><path d="M12 11v6M12 7h.01" /></>),
  back: <path d="M19 12H5M11 6l-6 6 6 6" />,
  refresh: <path d="M20 12a8 8 0 1 1-2.3-5.7M20 4v5h-5" />,
  search: (<><circle cx="11" cy="11" r="7" /><path d="M16 16l5 5" /></>),
  close: <path d="M6 6l12 12M18 6L6 18" />,
  chevronLeft: <path d="M15 6l-6 6 6 6" />,
  chevronRight: <path d="M9 6l6 6-6 6" />,
  logout: <path d="M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10" />,
  copy: (<><rect x="8" y="8" width="12" height="12" rx="1" /><path d="M4 16V4h12" /></>),
  check: <path d="M5 12l5 5 9-10" />,
  warning: <path d="M12 3l10 18H2zM12 10v5M12 18h.01" />,
  error: (<><circle cx="12" cy="12" r="9" /><path d="M9 9l6 6M15 9l-6 6" /></>),
  plus: <path d="M12 5v14M5 12h14" />,
  trash: <path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" />,
  more: (<><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></>),
} satisfies Record<string, ReactNode>;

export type IconName = keyof typeof PATHS;

export interface IconProps { name: IconName; size?: number; className?: string }

export function Icon({ name, size = 18, className }: IconProps) {
  return (
    <svg
      className={className ? `icon ${className}` : 'icon'}
      width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor"
      strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" focusable="false"
    >
      {PATHS[name]}
    </svg>
  );
}
```

Create `frontend/src/app/features.ts`:

```ts
import type { IconName } from '../components/Icon';
import { navCopy } from '../i18n/copy';

export type Phase = 0 | 1 | 2 | 3 | 4 | 5 | 6;

function toPhase(raw: string | undefined): Phase {
  const n = Number(raw ?? 0);
  return (Number.isInteger(n) && n >= 0 && n <= 6 ? n : 0) as Phase;
}

export const BUILD_PHASE: Phase = toPhase(import.meta.env.VITE_PHASE);

export type NavId = 'home' | 'scalper' | 'strategies' | 'builder' | 'live' | 'scanners' | 'newScanner' | 'scannerAlerts';

export interface NavEntry { id: NavId; label: string; path: string; icon: IconName; phase: Phase; shortcutId: string }

// FR-SHELL-002 order; phase per ROADMAP §7 GATE row for FR-SHELL-002.
export const NAV_ENTRIES: readonly NavEntry[] = [
  { id: 'home', label: navCopy.home, path: '/home', icon: 'home', phase: 0, shortcutId: 'nav.1' },
  { id: 'scalper', label: navCopy.scalper, path: '/scalper', icon: 'scalper', phase: 6, shortcutId: 'nav.2' },
  { id: 'strategies', label: navCopy.strategies, path: '/strategies', icon: 'strategies', phase: 2, shortcutId: 'nav.3' },
  { id: 'builder', label: navCopy.builder, path: '/builder', icon: 'builder', phase: 1, shortcutId: 'nav.4' },
  { id: 'live', label: navCopy.live, path: '/live', icon: 'live', phase: 4, shortcutId: 'nav.5' },
  { id: 'scanners', label: navCopy.scanners, path: '/scanners', icon: 'scanners', phase: 3, shortcutId: 'nav.6' },
  { id: 'newScanner', label: navCopy.newScanner, path: '/scanner', icon: 'newScanner', phase: 3, shortcutId: 'nav.7' },
  { id: 'scannerAlerts', label: navCopy.scannerAlerts, path: '/scanner-alerts', icon: 'alerts', phase: 3, shortcutId: 'nav.8' },
];

export function visibleNavEntries(phase: Phase = BUILD_PHASE): NavEntry[] {
  return NAV_ENTRIES.filter((e) => phase >= e.phase);
}

export type Feature = 'notificationsBell' | 'orderButtons' | 'liveStrategyCard' | 'profileToggles';

const FEATURE_PHASE: Record<Feature, Phase> = {
  notificationsBell: 3, // FR-SHELL-006 → FR-SHELL-003 GATE
  orderButtons: 4, // FR-CHART-012 / FR-ORD-012 GATE; trading disabled before P4
  liveStrategyCard: 4, // FR-HOME-003
  profileToggles: 3, // FR-ACCT-001 → FR-ACCT-003 (P3), FR-ACCT-002 (P4)
};

export function isEnabled(feature: Feature, phase: Phase = BUILD_PHASE): boolean {
  return phase >= FEATURE_PHASE[feature];
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/i18n src/design src/app/features.test.ts`
Expected: PASS (all tests in 4 files).

- [ ] **Step 6: Write the failing tests for theme, sidebar and shell**

Create `frontend/src/app/theme.test.tsx`:

```tsx
import { act, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { colourTokens } from '../design/tokens';
import { setSystemDark } from '../test/matchMedia';
import { ThemeProvider, useTheme } from './theme';

function Probe() {
  const { theme, tokens } = useTheme();
  return <output data-testid="probe">{`${theme}:${tokens.bg}`}</output>;
}

const root = () => document.documentElement;

describe('FR-SHELL-011 theme', () => {
  it('AC1 light by default writes every colour token as a CSS variable', () => {
    render(<ThemeProvider mode="light" followSystem={false}><Probe /></ThemeProvider>);
    expect(root().dataset.theme).toBe('light');
    expect(root().style.getPropertyValue('--surface-2')).toBe(colourTokens.light.surface2);
    expect(screen.getByTestId('probe')).toHaveTextContent(`light:${colourTokens.light.bg}`);
  });
  it('AC2 switching to dark applies within the same commit', () => {
    const { rerender } = render(<ThemeProvider mode="light" followSystem={false}><Probe /></ThemeProvider>);
    rerender(<ThemeProvider mode="dark" followSystem={false}><Probe /></ThemeProvider>);
    expect(root().dataset.theme).toBe('dark');
    expect(root().style.getPropertyValue('--bg')).toBe(colourTokens.dark.bg);
    expect(screen.getByTestId('probe')).toHaveTextContent(`dark:${colourTokens.dark.bg}`);
  });
  it('follows the desktop preference when "Follow system theme" is on', () => {
    render(<ThemeProvider mode="light" followSystem><Probe /></ThemeProvider>);
    expect(root().dataset.theme).toBe('light');
    act(() => setSystemDark(true));
    expect(root().dataset.theme).toBe('dark');
  });
});
```

Create `frontend/src/app/Sidebar.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';

function Where() {
  return <output data-testid="where">{useLocation().pathname}</output>;
}

function renderSidebar(collapsed = false) {
  const onToggle = vi.fn();
  const router = createMemoryRouter(
    [{ path: '*', element: (<><Sidebar entries={visibleNavEntries(6)} collapsed={collapsed} onToggleCollapsed={onToggle} /><Where /></>) }],
    { initialEntries: ['/home'] },
  );
  render(<RouterProvider router={router} />);
  return { onToggle };
}

describe('FR-SHELL-002 sidebar', () => {
  it('lists the entries with icon + label and marks the current one', () => {
    renderSidebar();
    expect(screen.getAllByRole('link')).toHaveLength(8);
    expect(screen.getByRole('link', { name: 'Home' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: 'Scalper' })).not.toHaveAttribute('aria-current');
  });
  it('AC3 Up/Down move focus between entries and Enter navigates', async () => {
    const user = userEvent.setup();
    renderSidebar();
    const links = screen.getAllByRole('link');
    links[0].focus();
    await user.keyboard('{ArrowDown}');
    expect(links[1]).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}');
    expect(links[7]).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(screen.getByTestId('where')).toHaveTextContent('/scanner-alerts');
  });
  it('collapse button reports the toggle and keeps accessible names when collapsed', async () => {
    const user = userEvent.setup();
    const { onToggle } = renderSidebar(true);
    expect(screen.getByRole('link', { name: 'Home' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Expand sidebar' }));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });
});
```

Create `frontend/src/app/AppShell.test.tsx`:

```tsx
import { render, screen, within } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { describe, expect, it } from 'vitest';
import { HomeLayout } from './HomeLayout';
import { buildRoutes } from './routes';

describe('FR-SHELL-001 frame', () => {
  it('AC1 (P0 form): top bar, sidebar and content are visible; the sidebar holds the P0 entries', async () => {
    const router = createMemoryRouter(buildRoutes(), { initialEntries: ['/home'] });
    render(<RouterProvider router={router} />);
    expect(await screen.findByRole('banner')).toBeInTheDocument();
    const nav = screen.getByRole('navigation', { name: 'Main navigation' });
    expect(within(nav).getAllByRole('link').map((l) => l.textContent)).toEqual(['Home']);
    expect(screen.getByRole('main')).toBeInTheDocument();
  });
  it('HomeLayout renders two column scroll containers', () => {
    render(<HomeLayout topRow={<p>cards</p>} main={<p>chart</p>} right={<p>lists</p>} />);
    expect(screen.getByTestId('home-left')).toHaveTextContent('cardschart');
    expect(screen.getByTestId('home-right')).toHaveTextContent('lists');
  });
});
```

- [ ] **Step 7: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/app`
Expected: FAIL — `Failed to resolve import "./theme"`, `"./Sidebar"`, `"./HomeLayout"`, `"./routes"`.

- [ ] **Step 8: Implement theme, layout components, routes and entry point**

Create `frontend/src/app/theme.tsx`:

```tsx
import { createContext, useContext, useLayoutEffect, useMemo, useSyncExternalStore, type ReactNode } from 'react';
import { colourTokens, cssVarName, type ColourTokens, type ThemeName } from '../design/tokens';

export function applyTheme(name: ThemeName, root: HTMLElement = document.documentElement): void {
  const tokens = colourTokens[name];
  for (const key of Object.keys(tokens) as (keyof ColourTokens)[]) root.style.setProperty(cssVarName(key), tokens[key]);
  root.dataset.theme = name;
  root.style.colorScheme = name;
}

const DARK_QUERY = '(prefers-color-scheme: dark)';

function subscribeSystem(onChange: () => void): () => void {
  const mql = window.matchMedia?.(DARK_QUERY);
  if (!mql) return () => {};
  mql.addEventListener('change', onChange);
  return () => mql.removeEventListener('change', onChange);
}

function systemIsDark(): boolean {
  return window.matchMedia?.(DARK_QUERY).matches ?? false;
}

export interface ThemeContextValue { theme: ThemeName; tokens: ColourTokens }

const ThemeContext = createContext<ThemeContextValue>({ theme: 'light', tokens: colourTokens.light });

export interface ThemeProviderProps { mode: ThemeName; followSystem: boolean; children: ReactNode }

export function ThemeProvider({ mode, followSystem, children }: ThemeProviderProps) {
  const systemDark = useSyncExternalStore(subscribeSystem, systemIsDark, () => false);
  const theme: ThemeName = followSystem ? (systemDark ? 'dark' : 'light') : mode;
  useLayoutEffect(() => applyTheme(theme), [theme]);
  const value = useMemo(() => ({ theme, tokens: colourTokens[theme] }), [theme]);
  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  return useContext(ThemeContext);
}
```

Create `frontend/src/app/AppShell.tsx`:

```tsx
import type { ReactNode } from 'react';
import './shell.css';

export interface AppShellProps { topBar: ReactNode; sidebar: ReactNode; banner?: ReactNode; children: ReactNode }

export function AppShell({ topBar, sidebar, banner, children }: AppShellProps) {
  return (
    <div className="shell">
      <header className="shell__top">{topBar}</header>
      <div className="shell__side">{sidebar}</div>
      <main className="shell__content" id="main-content">
        {banner}
        {children}
      </main>
    </div>
  );
}
```

Create `frontend/src/app/TopBar.tsx`:

```tsx
import type { ReactNode } from 'react';
import { shellCopy } from '../i18n/copy';

export interface TopBarProps { ticker?: ReactNode; status?: ReactNode; links?: ReactNode; profile?: ReactNode }

export function TopBar({ ticker, status, links, profile }: TopBarProps) {
  return (
    <div className="topbar">
      <div className="topbar__ticker" data-testid="topbar-ticker">{ticker}</div>
      <div className="topbar__status">{status}</div>
      <div className="topbar__spacer" />
      <nav className="topbar__links" aria-label={shellCopy.quickLinks}>{links}</nav>
      <div className="topbar__profile">{profile}</div>
    </div>
  );
}
```

Create `frontend/src/app/Sidebar.tsx`:

```tsx
import { useRef, type KeyboardEvent } from 'react';
import { NavLink } from 'react-router-dom';
import { Icon } from '../components/Icon';
import { shellCopy } from '../i18n/copy';
import type { NavEntry } from './features';

export interface SidebarProps { entries: readonly NavEntry[]; collapsed: boolean; onToggleCollapsed: () => void }

export function Sidebar({ entries, collapsed, onToggleCollapsed }: SidebarProps) {
  const links = useRef<(HTMLAnchorElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent<HTMLAnchorElement>, index: number) => {
    if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') return;
    e.preventDefault();
    const n = entries.length;
    const next = (index + (e.key === 'ArrowDown' ? 1 : -1) + n) % n;
    links.current[next]?.focus();
  };

  return (
    <nav className={collapsed ? 'sidebar sidebar--collapsed' : 'sidebar'} aria-label={shellCopy.sidebarLabel}>
      <ul>
        {entries.map((entry, i) => (
          <li key={entry.id}>
            <NavLink
              ref={(el) => { links.current[i] = el; }}
              to={entry.path}
              className="sidebar__link"
              aria-label={collapsed ? entry.label : undefined}
              onKeyDown={(e) => onKeyDown(e, i)}
            >
              <Icon name={entry.icon} />
              <span className="sidebar__label">{entry.label}</span>
            </NavLink>
          </li>
        ))}
      </ul>
      <button
        type="button"
        className="sidebar__collapse"
        onClick={onToggleCollapsed}
        aria-label={collapsed ? shellCopy.expandSidebar : shellCopy.collapseSidebar}
        aria-expanded={!collapsed}
      >
        <Icon name={collapsed ? 'chevronRight' : 'chevronLeft'} />
      </button>
    </nav>
  );
}
```

Create `frontend/src/app/HomeLayout.tsx`:

```tsx
import type { ReactNode } from 'react';
import { homeCopy } from '../i18n/copy';

export interface HomeLayoutProps { topRow?: ReactNode; main: ReactNode; below?: ReactNode; right: ReactNode }

export function HomeLayout({ topRow, main, below, right }: HomeLayoutProps) {
  return (
    <div className="home">
      <div className="home__left" data-testid="home-left">
        {topRow && <div className="home__top-row">{topRow}</div>}
        <div className="home__main">{main}</div>
        {below}
      </div>
      <aside className="home__right" data-testid="home-right" aria-label={homeCopy.rightColumn}>
        {right}
      </aside>
    </div>
  );
}
```

Create `frontend/src/app/shell.css`:

```css
.shell {
  display: grid; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: var(--topbar-h) minmax(0, 1fr);
  grid-template-areas: "top top" "side content"; height: 100vh; min-width: 1280px; background: var(--bg); color: var(--text);
}
.shell__top { grid-area: top; min-width: 0; background: var(--surface); border-bottom: 1px solid var(--border); }
.shell__side { grid-area: side; min-height: 0; overflow-y: auto; background: var(--surface); border-right: 1px solid var(--border); }
.shell__content { grid-area: content; min-width: 0; min-height: 0; overflow: auto; display: flex; flex-direction: column; }
.shell__content > :last-child { flex: 1 1 auto; min-height: 0; }

.topbar { display: flex; align-items: center; gap: var(--space-3); height: 100%; padding: 0 var(--space-3); min-width: 0; }
.topbar__ticker { display: flex; gap: var(--space-2); flex: 0 1 auto; min-width: 0; overflow: hidden; }
.topbar__status, .topbar__links, .topbar__profile { flex: none; display: flex; align-items: center; gap: var(--space-2); }
.topbar__spacer { flex: 1 1 0; }

.sidebar { display: flex; flex-direction: column; height: 100%; width: var(--sidebar-w); transition: width var(--motion-fast); }
.sidebar--collapsed { width: var(--sidebar-w-collapsed); }
.sidebar ul { list-style: none; margin: 0; padding: var(--space-2); display: flex; flex-direction: column; gap: var(--space-1); flex: 1 1 auto; }
.sidebar__link {
  display: flex; align-items: center; gap: var(--space-3); padding: var(--space-2) var(--space-3); border-radius: var(--radius-1);
  color: var(--text); text-decoration: none; white-space: nowrap; overflow: hidden;
}
.sidebar__link:hover { background: var(--surface-2); }
.sidebar__link[aria-current="page"] { background: var(--surface-2); color: var(--accent); font-weight: 600; }
.sidebar--collapsed .sidebar__label { display: none; }
.sidebar__collapse { margin: var(--space-2); align-self: flex-end; }

.home { display: grid; grid-template-columns: minmax(0, 2fr) minmax(320px, 1fr); gap: var(--space-4); padding: var(--space-4); height: 100%; min-height: 0; }
.home__left, .home__right { min-height: 0; overflow-y: auto; display: flex; flex-direction: column; gap: var(--space-4); }
.home__top-row { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: var(--space-4); }
.home__main { min-height: 360px; flex: 1 0 auto; display: flex; flex-direction: column; }
```

Create `frontend/src/app/ShellLayout.tsx`:

```tsx
import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { AppShell } from './AppShell';
import { visibleNavEntries } from './features';
import { Sidebar } from './Sidebar';
import { TopBar } from './TopBar';

export function ShellLayout() {
  const [collapsed, setCollapsed] = useState(false);
  return (
    <AppShell
      topBar={<TopBar ticker={null} />}
      sidebar={<Sidebar entries={visibleNavEntries()} collapsed={collapsed} onToggleCollapsed={() => setCollapsed((c) => !c)} />}
    >
      <Outlet />
    </AppShell>
  );
}
```

Create `frontend/src/app/RootLayout.tsx`:

```tsx
import { Outlet } from 'react-router-dom';

export function RootLayout() {
  return <Outlet />;
}
```

Create `frontend/src/views/home/HomeView.tsx`:

```tsx
import { HomeLayout } from '../../app/HomeLayout';
import { homeCopy } from '../../i18n/copy';

export function HomeView() {
  return (
    <>
      <h1 className="visually-hidden">{homeCopy.title}</h1>
      <HomeLayout
        main={<section className="card" aria-label={homeCopy.chartArea} />}
        right={<section className="card" aria-label={homeCopy.watchlistArea} />}
      />
    </>
  );
}
```

Create `frontend/src/app/routes.tsx`:

```tsx
import { Navigate, type RouteObject } from 'react-router-dom';
import { HomeView } from '../views/home/HomeView';
import { RootLayout } from './RootLayout';
import { ShellLayout } from './ShellLayout';

/** Views rendered inside the shell frame. Later tasks (and P0-5) append entries here. */
export const shellChildren: RouteObject[] = [
  { path: 'home', element: <HomeView /> },
];

/** Children of the root layout that render outside the shell (onboarding) plus the shell itself. */
export const rootChildren: RouteObject[] = [
  { index: true, element: <Navigate to="/home" replace /> },
];

export function buildRoutes(): RouteObject[] {
  return [
    {
      path: '/',
      element: <RootLayout />,
      children: [...rootChildren, { element: <ShellLayout />, children: [...shellChildren] }],
    },
  ];
}
```

Create `frontend/src/app/router.ts`:

```ts
import { createMemoryRouter } from 'react-router-dom';
import { buildRoutes } from './routes';

export type AppRouter = ReturnType<typeof createMemoryRouter>;

export function createAppRouter(initialEntries: string[] = ['/']): AppRouter {
  return createMemoryRouter(buildRoutes(), { initialEntries });
}
```

Create `frontend/src/app/App.tsx`:

```tsx
import { useState } from 'react';
import { RouterProvider } from 'react-router-dom';
import { createAppRouter, type AppRouter } from './router';
import { ThemeProvider } from './theme';

export interface AppProps { router?: AppRouter }

export function App({ router }: AppProps) {
  const [r] = useState(() => router ?? createAppRouter());
  return (
    <ThemeProvider mode="light" followSystem={false}>
      <RouterProvider router={r} />
    </ThemeProvider>
  );
}
```

Overwrite `frontend/src/main.tsx`:

```tsx
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './design/tokens.css';
import './design/base.css';
import { App } from './app/App';
import { applyTheme } from './app/theme';

applyTheme('light');
const root = document.getElementById('root');
if (!root) throw new Error('index.html must contain <div id="root">');
createRoot(root).render(
  <StrictMode>
    <App />
  </StrictMode>,
);
```

- [ ] **Step 9: Run the whole suite and the type check**

Run: `cd frontend && npx vitest run && npm run typecheck && npm run lint`
Expected: PASS (7 test files), `tsc` exits 0, `eslint` prints nothing and exits 0.

- [ ] **Step 10: Add the 1280×720 Playwright smoke test and run it**

Create `frontend/e2e/shell.spec.ts`:

```ts
import { expect, test } from '@playwright/test';

test('FR-SHELL-001 AC2 (frame part): no horizontal scrollbar at 1280×720; bars inside the window', async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto('/');
  const nav = page.getByRole('navigation', { name: 'Main navigation' });
  await expect(nav).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBe(0);
  for (const box of [await page.getByRole('banner').boundingBox(), await nav.boundingBox()]) {
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(1280);
  }
});
```

Run: `cd frontend && npx playwright test e2e/shell.spec.ts`
Expected: PASS (1 test × 2 projects: webkit, chromium).

- [ ] **Step 11: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/eslint.config.js frontend/vite.config.ts frontend/.env.mock \
  frontend/playwright.config.ts frontend/tsconfig.json frontend/e2e frontend/src
git commit -m "feat(frontend): app shell layout, theme tokens and Indian formatters (P0-4 Task 1)" \
  -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
