/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_BACKEND?: 'mock' | 'wails';
  readonly VITE_PHASE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
