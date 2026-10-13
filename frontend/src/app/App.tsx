import { useState, type ReactNode } from 'react';
import { RouterProvider } from 'react-router-dom';
import type { Backend } from '../api/backend';
import { BackendProvider } from '../api/BackendContext';
import type { ThemeName } from '../design/tokens';
import { MarketDataBridge } from '../market/MarketDataBridge';
import { useSetting } from '../store/settings';
import { createAppRouter, type AppRouter } from './router';
import { ThemeProvider } from './theme';

export interface AppProps { backend: Backend; router?: AppRouter }

function ConnectedTheme({ children }: { children: ReactNode }) {
  const [mode] = useSetting<ThemeName>('appearance.theme', 'light');
  const [followSystem] = useSetting<boolean>('appearance.follow_system', false);
  return <ThemeProvider mode={mode} followSystem={followSystem}>{children}</ThemeProvider>;
}

export function App({ backend, router }: AppProps) {
  const [r] = useState(() => router ?? createAppRouter());
  return (
    <BackendProvider backend={backend}>
      <MarketDataBridge />
      <ConnectedTheme>
        <RouterProvider router={r} />
      </ConnectedTheme>
    </BackendProvider>
  );
}
