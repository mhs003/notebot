import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // emptyOutDir is off so the committed .gitkeep placeholder survives, which
  // keeps `go build ./...` working on a fresh clone before any web build.
  // Stale assets are cleared by the "prebuild" script in package.json.
  build: { outDir: 'dist', emptyOutDir: false },
  server: { proxy: { '/api': 'http://127.0.0.1:8765' } },
})
