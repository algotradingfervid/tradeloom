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
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/chart/LightweightChartsAdapter.ts', 'src/chart/lwc.ts', 'src/chart/time.ts',
      'src/chart/primitives/**', 'src/chart/testing/**', 'src/chart/**/*.test.ts',
    ],
    rules: {
      'no-restricted-imports': ['error', { paths: [{
        name: 'lightweight-charts',
        message: 'Use ChartAdapter (src/chart/ChartAdapter.ts); only the adapter may import lightweight-charts (FR-CHART-001).',
      }] }],
    },
  },
]);
