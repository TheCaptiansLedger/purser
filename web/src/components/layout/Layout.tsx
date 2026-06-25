import { useCallback, useEffect, useState } from 'react'
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'

const SIDEBAR_KEY = 'sidebar-collapsed'

export function parseSidebarCollapsed(stored: string | null): boolean {
  return stored === 'true'
}

export function Layout() {
  const [collapsed, setCollapsed] = useState(() => parseSidebarCollapsed(localStorage.getItem(SIDEBAR_KEY)))

  const toggleCollapsed = useCallback((value: boolean) => {
    setCollapsed(value)
    localStorage.setItem(SIDEBAR_KEY, String(value))
  }, [])

  // Expose sidebar width as a CSS custom property so fixed children (e.g. AfterDark tab bar) can offset correctly.
  useEffect(() => {
    document.documentElement.style.setProperty('--sidebar-width', collapsed ? '4rem' : '15rem')
  }, [collapsed])

  return (
    <div className="flex h-full">
      <Sidebar collapsed={collapsed} onCollapsedChange={toggleCollapsed} />
      <main
        className="flex-1 min-h-screen overflow-y-auto transition-all duration-300"
        style={{ marginLeft: collapsed ? '4rem' : '15rem' }}
      >
        <Outlet />
      </main>
    </div>
  )
}
