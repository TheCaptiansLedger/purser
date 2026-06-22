import type { ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { useSetupStatus } from '../../api/setup'

export function resolveSetupRedirect(complete: boolean, pathname: string): string | null {
  if (!complete && pathname !== '/setup') return '/setup'
  if (complete && pathname === '/setup') return '/'
  return null
}

export function SetupGuard({ children }: { children: ReactNode }) {
  const location = useLocation()
  const { data, isLoading } = useSetupStatus()

  if (isLoading) return null

  if (data !== undefined) {
    const target = resolveSetupRedirect(data.complete, location.pathname)
    if (target) return <Navigate to={target} replace />
  }

  return <>{children}</>
}
