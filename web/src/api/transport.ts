import { createConnectTransport } from '@connectrpc/connect-web'

// baseUrl is '/' (same-origin) in both environments this app runs in:
// - dev: Vite's proxy (vite.config.ts) forwards every /purser.* path to
//   the Go server, so the browser never sees a cross-origin request.
// - prod: cmd/purser serves the built web/dist itself (see
//   docs/design/frontend-stack.md), so the API and the UI share an
//   origin for real, not just via a proxy.
// Neither path needs CORS handling anywhere — see that doc's Data-layer
// section.
export const transport = createConnectTransport({
  baseUrl: '/',
})
