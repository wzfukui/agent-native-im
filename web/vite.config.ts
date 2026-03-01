import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      // WebSocket must come before /api to match first
      '/api/v1/ws': {
        target: 'http://localhost:9800',
        ws: true,
      },
      '/api': 'http://localhost:9800',
    },
  },
})
