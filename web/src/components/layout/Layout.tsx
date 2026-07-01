import { useCallback, useEffect, useState } from 'react'
import { Outlet, useLocation } from 'react-router-dom'
import { Menu } from 'lucide-react'
import { Sidebar } from './Sidebar'

const SIDEBAR_KEY = 'sidebar-collapsed'

export function parseSidebarCollapsed(stored: string | null): boolean {
  return stored === 'true'
}

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

  // Sidebar width as a CSS custom property — 0 on mobile so fixed children (tab bars, etc.) align to viewport edge.
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
    <div className="flex h-full">
      <Sidebar
        collapsed={collapsed}
        onCollapsedChange={toggleCollapsed}
        mobileOpen={mobileOpen}
      />

      {mobileOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/60 md:hidden"
          onClick={() => setMobileOpen(false)}
        />
      )}

      <main
        className="flex-1 min-h-screen overflow-y-auto transition-all duration-300"
        style={{ marginLeft: 'var(--sidebar-width)' }}
      >
        <div
          className="sticky top-0 z-20 flex items-center h-12 px-4 border-b border-white/5 md:hidden"
          style={{ background: 'rgba(5,5,12,0.95)', backdropFilter: 'blur(8px)' }}
        >
          <button
            onClick={() => setMobileOpen(true)}
            className="text-white/50 hover:text-white/80 transition-colors"
            aria-label="Open navigation"
          >
            <Menu size={20} />
          </button>
        </div>
        <Outlet />
      </main>
    </div>
  )
}
