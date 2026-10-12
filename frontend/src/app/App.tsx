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
