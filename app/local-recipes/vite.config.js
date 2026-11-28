import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// In production the Go server serves build/ and the API from the same
// origin, behind the identity gate. The dev server proxies /api to the Go
// backend and injects the identity header the gate would normally add, so
// local development works without SSO.
const apiTarget = process.env.API_PROXY_TARGET || 'http://localhost:8080'
const devUser = process.env.DEV_USER || 'dev-user'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'build',
    sourcemap: false,
    rollupOptions: {
      output: {
        manualChunks: {
          react: ['react', 'react-dom', 'react-router-dom'],
        },
      },
    },
  },
  server: {
    host: true,
    port: 3000,
    // Bind-mounted source in docker compose doesn't deliver fs events.
    watch: process.env.CHOKIDAR_USEPOLLING ? { usePolling: true } : undefined,
    proxy: {
      '/api': {
        target: apiTarget,
        changeOrigin: true,
        headers: {
          'X-Auth-Request-User': devUser,
          'X-Auth-Request-Email': `${devUser}@example.com`,
        },
      },
    },
  },
})
