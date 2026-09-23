import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// The build lands directly inside the Go package that embeds it, so no copy
// step is needed. `emptyOutDir` keeps the output free of stale hashed bundles.
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: '../internal/app/web/dist',
    emptyOutDir: true,
    chunkSizeWarningLimit: 800,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://127.0.0.1:8787',
      '/healthz': 'http://127.0.0.1:8787',
      '/readyz': 'http://127.0.0.1:8787',
      '/manifest.json': 'http://127.0.0.1:8787',
      '/download': 'http://127.0.0.1:8787',
      '/files': 'http://127.0.0.1:8787',
    },
  },
})
