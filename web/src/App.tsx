import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TransportProvider } from '@connectrpc/connect-query'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { transport } from './api/transport'
import { Layout } from './components/layout/Layout'
import { Welcome } from './pages/Welcome'

const queryClient = new QueryClient()

// Single route today — no module (Music/AfterDark/Acquisition/
// Pipeline) screens exist yet. See docs/design/frontend-stack.md.
const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [{ path: '/', element: <Welcome /> }],
  },
])

function App() {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </TransportProvider>
  )
}

export default App
