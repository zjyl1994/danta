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
        manualChunks(id) {
          // Vite 虚拟模块（preload-helper 等）跟随引用方，避免跨 chunk 强制依赖成环
          if (id.startsWith('\0')) return undefined
          if (!id.includes('node_modules')) return 'app'
          // MUI 及其底层样式/动画依赖
          if (/(?:^|[\\/])@mui[\\/]/.test(id) || /(?:^|[\\/])@emotion[\\/]/.test(id) || /(?:^|[\\/])@popperjs[\\/]/.test(id)) return 'mui'
          // React 运行时与路由
          if (/(?:^|[\\/])@remix-run[\\/]/.test(id)) return 'react'
          if (
            /(?:^|[\\/])(react-router-dom|react-router|react-dom|react-dropzone|react-transition-group|react-is|react|scheduler|hoist-non-react-statics|prop-types|object-assign|loose-envify|js-tokens|clsx|csstype|dom-helpers|attr-accept|file-selector)[\\/]/.test(id)
          ) {
            return 'react'
          }
          return 'vendor'
        }
      }
    }
  }
})
