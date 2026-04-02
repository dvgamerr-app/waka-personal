import { defineConfig } from 'astro/config'
import { fileURLToPath } from 'node:url'
import react from '@astrojs/react'
import tailwindcss from '@tailwindcss/vite'
import { loadEnv } from 'vite'

const env = loadEnv(process.env.NODE_ENV || 'development', process.cwd(), '')
const apiProxyTarget = env.API_PROXY_TARGET || 'http://127.0.0.1:8080'

const silenceWatcherListenerWarning = () => ({
  name: 'silence-watcher-listener-warning',
  configureServer(server) {
    server.watcher.setMaxListeners(20)
  },
})

export default defineConfig({
  integrations: [react()],
  output: 'static',
  server: {
    host: false,
  },
  vite: {
    plugins: [silenceWatcherListenerWarning(), tailwindcss()],
    optimizeDeps: {
      include: [
        'react',
        'react-dom',
        'react-dom/client',
        'react/jsx-runtime',
        'react/jsx-dev-runtime',
      ],
    },
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      proxy: {
        '/api': {
          target: apiProxyTarget,
          changeOrigin: true,
        },
      },
      watch: {
        ignored: ['.github/**/*', '.vscode/**/*'],
      },
    },
  },
  security: {
    checkOrigin: false,
  },
})
