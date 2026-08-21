import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:8080'
    }
  },
  build: {
    outDir: 'dist',
    chunkSizeWarningLimit: 600,
    rollupOptions: {
      output: {
        // 第三方依赖（含 CJS interop 辅助模块）统一进 vendor，避免跨 chunk 循环依赖
        manualChunks(id) {
          if (id.startsWith('\0') || id.includes('node_modules')) return 'vendor'
        }
      }
    }
  }
})
