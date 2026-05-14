/// <reference types="vitest" />
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
    },
  },
  test: {
    environment: 'happy-dom',
    setupFiles: ['./test/vitest.setup.ts'],
    css: false,
    // Playwright owns everything under test/e2e/. Vitest's default glob
    // matches *.spec.ts, which would otherwise try (and fail) to run
    // Playwright specs.
    exclude: ['node_modules', 'dist', 'test/e2e/**'],
  },
});
