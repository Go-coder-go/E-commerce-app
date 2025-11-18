import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      // proxy any path starting with /products, /cart, /checkout to backend on :8082
      '/products': { target: 'http://localhost:8082', changeOrigin: true, secure: false },
      '/cart':     { target: 'http://localhost:8082', changeOrigin: true, secure: false },
      '/checkout': { target: 'http://localhost:8082', changeOrigin: true, secure: false },
    }
  }
})
