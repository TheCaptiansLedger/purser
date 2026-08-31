import { useCallback, useEffect, useState } from 'react'
import { Menu } from 'lucide-react'
import { Outlet, useLocation } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { PlayerProvider } from '../PlayerProvider'
import { NowPlayingBar } from '../NowPlayingBar'

const SIDEBAR_KEY = 'sidebar-collapsed'

export function parseSidebarCollapsed(stored: string | null): boolean {
  return stored === 'true'
}

// Layout is the app's one fixed-width element (the sidebar) plus a fluid
// content area offset by it — see
// docs/design/style-guide.md#fluid-by-default--no-centered-container and
// docs/design/frontend-stack.md#styling-tailwind-v4. This is the same
// shape this project's own pre-reset prior art used
// (--sidebar-width custom property, margin-left offset, off-canvas
// drawer below md).
export function Layout() {
  const [collapsed, setCollapsed] = useState(() => parseSidebarCollapsed(localStorage.getItem(SIDEBAR_KEY)))
  const [mobileOpen, setMobileOpen] = useState(false)
  const location = useLocation()

  const toggleCollapsed = useCallback((value: boolean) => {
    setCollapsed(value)
    localStorage.setItem(SIDEBAR_KEY, String(value))
  }, [])

  useEffect(() => {
    setMobileOpen(false)
  }, [location.pathname])

  // --sidebar-width is 0 below md (the sidebar is an off-canvas drawer
  // there, not a layout-affecting element), so main's margin-left offset
  // only applies once the sidebar is actually a persistent rail.
  useEffect(() => {
    const update = () => {
      const isMobile = window.innerWidth < 768
      document.documentElement.style.setProperty(
        '--sidebar-width',
        isMobile ? '0px' : collapsed ? '4rem' : '15rem',
      )
    }
    update()
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [collapsed])

  return (
    <PlayerProvider>
      <div className="flex min-h-screen bg-bg">
        <Sidebar collapsed={collapsed} onCollapsedChange={toggleCollapsed} mobileOpen={mobileOpen} />

        {mobileOpen && (
          <button
            type="button"
            aria-label="Close navigation"
            className="fixed inset-0 z-30 bg-black/60 md:hidden"
            onClick={() => setMobileOpen(false)}
          />
        )}

        <main
          className="flex-1 min-h-screen overflow-y-auto transition-all duration-300"
          style={{ marginLeft: 'var(--sidebar-width)', paddingBottom: 'var(--player-bar-height, 0px)' }}
        >
          <div className="sticky top-0 z-20 flex items-center h-12 px-4 border-b border-border bg-surface/95 backdrop-blur-xl md:hidden">
            <button
              onClick={() => setMobileOpen(true)}
              className="text-text-secondary hover:text-text transition-colors"
              aria-label="Open navigation"
            >
              <Menu size={20} />
            </button>
          </div>
          <Outlet />
        </main>

        <NowPlayingBar />
      </div>
    </PlayerProvider>
  )
}
