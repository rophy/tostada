/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import istanbul from 'vite-plugin-istanbul'

export default defineConfig({
  plugins: [
    react(),
    ...(process.env.DEVELOPMENT === '1' ? [istanbul({ include: 'src/**', exclude: ['src/__tests__/**', 'node_modules/**'], forceBuildInstrument: true })] : []),
  ],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test-setup.ts',
    coverage: {
      include: ['src/**'],
      exclude: ['src/__tests__/**', 'src/test-setup.ts'],
    },
  },
})
