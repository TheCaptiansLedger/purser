import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { TransportProvider } from '@connectrpc/connect-query'
import { Navigate, createBrowserRouter, RouterProvider } from 'react-router-dom'
import { transport } from './api/transport'
import { Layout } from './components/layout/Layout'
import { Welcome } from './pages/Welcome'
import { People } from './pages/People'
import { PersonDetail } from './pages/PersonDetail'
import { SettingsLayout } from './pages/settings/SettingsLayout'
import { ConfigTab } from './pages/settings/ConfigTab'
import { JobsTab } from './pages/settings/JobsTab'
import { DatabaseTab } from './pages/settings/DatabaseTab'
import { CacheTab } from './pages/settings/CacheTab'

const queryClient = new QueryClient()

// Welcome, /people (#660), /people/:id (#661), and the /settings/*
// admin area (#596). No other module (Music/AfterDark/Acquisition/Pipeline)
// screens exist yet.
// See docs/design/frontend-stack.md.
const router = createBrowserRouter([
  {
    element: <Layout />,
    children: [
      { path: '/', element: <Welcome /> },
      { path: '/people', element: <People /> },
      { path: '/people/:id', element: <PersonDetail /> },
      {
        path: '/settings',
        element: <SettingsLayout />,
        children: [
          { index: true, element: <Navigate to="config" replace /> },
          { path: 'config', element: <ConfigTab /> },
          { path: 'jobs', element: <JobsTab /> },
          { path: 'database', element: <DatabaseTab /> },
          { path: 'cache', element: <CacheTab /> },
        ],
      },
    ],
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
