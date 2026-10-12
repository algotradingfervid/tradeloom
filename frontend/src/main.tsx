import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import './design/tokens.css';
import './design/base.css';
import { defaultBackend } from './api';
import { App } from './app/App';
import { applyTheme } from './app/theme';

applyTheme('light');
const root = document.getElementById('root');
if (!root) throw new Error('index.html must contain <div id="root">');
createRoot(root).render(
  <StrictMode>
    <App backend={defaultBackend()} />
  </StrictMode>,
);
