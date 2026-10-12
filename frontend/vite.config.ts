import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// Wails serves frontend/dist from the embedded FS (frontend/assets.go).
export default defineConfig({
  plugins: [react()],
  build: { outDir: 'dist', emptyOutDir: true },
  test: { environment: 'jsdom', globals: true },
})
