import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8081', // Backend server
        changeOrigin: true,
        // rewrite: (path) => path.replace(/^\/api/, '') // Don't rewrite if backend is also /api
      }
    }
  }
})
