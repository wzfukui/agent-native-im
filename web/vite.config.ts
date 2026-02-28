import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/api': 'http://localhost:9800',
      '/api/v1/ws': {
        target: 'ws://localhost:9800',
        ws: true,
      },
    },
  },
})
