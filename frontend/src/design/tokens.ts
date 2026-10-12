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
