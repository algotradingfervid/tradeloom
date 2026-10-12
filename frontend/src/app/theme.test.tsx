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
