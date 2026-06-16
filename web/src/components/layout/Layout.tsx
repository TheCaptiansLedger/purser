import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { useEffect, useState } from 'react'

const SIDEBAR_KEY = 'sidebar-collapsed'

export function Layout() {
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem(SIDEBAR_KEY) === 'true')

  useEffect(() => {
    const handler = () => setCollapsed(localStorage.getItem(SIDEBAR_KEY) === 'true')
    window.addEventListener('storage', handler)
    // Also poll on mount since Sidebar uses localStorage directly
    const id = setInterval(() => setCollapsed(localStorage.getItem(SIDEBAR_KEY) === 'true'), 200)
    return () => { window.removeEventListener('storage', handler); clearInterval(id) }
  }, [])

  // Expose sidebar width as a CSS custom property so fixed children (e.g. AfterDark tab bar) can offset correctly.
  useEffect(() => {
    document.documentElement.style.setProperty('--sidebar-width', collapsed ? '4rem' : '15rem')
  }, [collapsed])

  return (
    <div className="flex h-full">
      <Sidebar />
      <main
        className="flex-1 min-h-screen overflow-y-auto transition-all duration-300"
        style={{ marginLeft: collapsed ? '4rem' : '15rem' }}
      >
        <Outlet />
      </main>
    </div>
  )
}
