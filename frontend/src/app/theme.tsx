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
