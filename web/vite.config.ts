import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      // Every Connect RPC path is /<package>.<Service>/<Method> (e.g.
      // /purser.job.v1.JobService/ListJobs) — see
      // docs/design/frontend-stack.md#data-layer-connect-web-not-rest.
      // Proxying by that shape, rather than an /api prefix nothing
      // actually used, covers every current and future service without
      // naming them individually, and keeps the browser same-origin so
      // no CORS handling is needed on the Go server.
      '^/purser\\.': 'http://localhost:7474',
    },
  },
  test: {
    environment: 'jsdom',
    // jsdom only attaches window.localStorage for a real http(s) origin —
    // the default about:blank leaves it undefined, which Layout's
    // sidebar-collapsed persistence depends on.
    environmentOptions: { jsdom: { url: 'http://localhost/' } },
    globals: true,
    setupFiles: ['./src/setupTests.ts'],
  },
})
