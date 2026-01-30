import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      '/rest': 'http://localhost:8080',
      '/auth': 'http://localhost:8080',
      '/_/api': 'http://localhost:8080',
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  }
})
