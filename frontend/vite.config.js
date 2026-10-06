import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Build output goes straight into ../web which the Go backend serves.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../web',
    emptyOutDir: true,
    assetsDir: 'assets',
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
