import { defineConfig } from 'astro/config'
import { fileURLToPath } from 'node:url'
import svelte from '@astrojs/svelte'
import tailwindcss from '@tailwindcss/vite'
import { getAdapter as getBunAdapter } from '@nurodev/astro-bun'

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
    plugins: [tailwindcss()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server: {
      watch: {
        ignored: ['.github/**/*', '.vscode/**/*'],
      },
    },
  },
  security: {
    checkOrigin: false,
  },
})
