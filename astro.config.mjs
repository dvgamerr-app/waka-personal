import { defineConfig } from 'astro/config'
import { fileURLToPath } from 'node:url'
import svelte from '@astrojs/svelte'
import tailwindcss from '@tailwindcss/vite'
import { loadEnv } from 'vite'
import { getAdapter as getBunAdapter } from '@nurodev/astro-bun'

const env = loadEnv(process.env.NODE_ENV || 'development', process.cwd(), '')
const apiProxyTarget = env.API_PROXY_TARGET || 'http://127.0.0.1:8080'

const silenceWatcherListenerWarning = () => ({
  name: 'silence-watcher-listener-warning',
  configureServer(server) {
    server.watcher.setMaxListeners(20)
  },
})

const bunAdapter = (options = {}) => {
  let command = 'build'

  return {
    name: '@nurodev/astro-bun-patched',
    hooks: {
      'astro:config:setup': ({ command: astroCommand }) => {
        command = astroCommand
      },
      'astro:config:done': ({ config, setAdapter }) => {
        const adapter = getBunAdapter({
          ...options,
          assets: config.build.assets,
          client: config.build.client.href,
          host: config.server.host,
          port: config.server.port,
          server: config.build.server.href,
        })

        if (command === 'dev') {
          adapter.entrypointResolution = 'auto'
        }

        setAdapter(adapter)
      },
    },
  }
}

export default defineConfig({
  adapter: bunAdapter(),
  integrations: [svelte()],
  output: 'server',
  build: {
    assets: 'dist',
  },
  server: {
    host: false,
  },
  vite: {
    plugins: [silenceWatcherListenerWarning(), tailwindcss()],
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
