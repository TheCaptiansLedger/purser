import { NavLink, useLocation } from 'react-router-dom'
import { Users, Settings, ChevronLeft, ChevronRight, Hexagon, Loader2, Map } from 'lucide-react'
import { useModules } from '../../context/ModulesContext'
import { useJobs } from '../../api/jobs'
import { MODULE_REGISTRY } from '../../config/modules'

const BOTTOM_NAV = [
  { path: '/people',   label: 'People',   icon: Users    },
  { path: '/roadmap',  label: 'Roadmap',  icon: Map      },
  { path: '/settings', label: 'Settings', icon: Settings },
]

interface SidebarProps {
  collapsed: boolean
  onCollapsedChange: (collapsed: boolean) => void
  mobileOpen?: boolean
}

export function Sidebar({ collapsed, onCollapsedChange, mobileOpen }: SidebarProps) {
  const location = useLocation()
  const modules = useModules()

  const visibleNav = MODULE_REGISTRY.filter(m => modules[m.key])
  const activeAccent = visibleNav.find(m => location.pathname.startsWith(m.path))?.accent ?? '#6366f1'

  const { data: jobsData } = useJobs()
  const pendingCount = (jobsData?.data ?? []).filter(
    j => j.status === 'queued' || j.status === 'running'
  ).length

  return (
    <aside
      style={{ '--accent': activeAccent } as React.CSSProperties}
      className={[
        'fixed left-0 top-0 h-screen z-40 flex flex-col',
        'bg-[#05050c] border-r border-white/5',
        'transition-all duration-300 ease-in-out',
        collapsed ? 'w-16' : 'w-60',
        mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
      ].join(' ')}
    >
      {/* Logo */}
      <div className={['flex items-center gap-3 h-16 px-4 border-b border-white/5 shrink-0', collapsed ? 'justify-center' : ''].join(' ')}>
        <div className="shrink-0 w-8 h-8 rounded-lg flex items-center justify-center" style={{ background: 'linear-gradient(135deg, var(--accent), color-mix(in srgb, var(--accent) 60%, #000))' }}>
          <Hexagon size={16} className="text-white" strokeWidth={2.5} />
        </div>
        {!collapsed && (
          <span className="text-sm font-semibold tracking-widest text-white/90 uppercase">Purser</span>
        )}
      </div>

      {/* Main nav */}
      <nav className="flex-1 flex flex-col gap-0.5 px-2 py-3 overflow-y-auto overflow-x-hidden">
        {visibleNav.map(({ path, label, icon: Icon, accent }) => (
          <NavLink
            key={path}
            to={path}
            style={({ isActive }) => isActive ? { '--item-accent': accent } as React.CSSProperties : { '--item-accent': 'transparent' } as React.CSSProperties}
            className={({ isActive }) => [
              'group relative flex items-center gap-3 rounded-lg px-3 h-10 text-sm font-medium',
              'transition-all duration-150 cursor-pointer select-none',
              isActive
                ? 'text-white bg-white/8'
                : 'text-white/50 hover:text-white/80 hover:bg-white/5',
              collapsed ? 'justify-center' : '',
            ].join(' ')}
          >
            {({ isActive }) => (
              <>
                {/* Active indicator bar */}
                {isActive && (
                  <span
                    className="absolute left-0 top-1/2 -translate-y-1/2 w-0.5 h-5 rounded-full"
                    style={{ background: accent }}
                  />
                )}
                <Icon
                  size={18}
                  style={isActive ? { color: accent } : {}}
                  className="shrink-0 transition-colors duration-150"
                  strokeWidth={isActive ? 2.5 : 2}
                />
                {!collapsed && (
                  <span className="truncate">{label}</span>
                )}
                {collapsed && (
                  <span className="absolute left-full ml-3 px-2 py-1 rounded-md bg-zinc-800 text-white text-xs font-medium
                    opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity duration-150 whitespace-nowrap shadow-lg">
                    {label}
                  </span>
                )}
              </>
            )}
          </NavLink>
        ))}
      </nav>

      {/* Divider */}
      <div className="mx-3 border-t border-white/5" />

      {/* Active jobs indicator */}
      {pendingCount > 0 && (
        <nav className="px-2 pt-2">
          <NavLink
            to="/settings/jobs"
            className={({ isActive }) => [
              'group relative flex items-center gap-3 rounded-lg px-3 h-10 text-sm font-medium',
              'transition-all duration-150',
              isActive
                ? 'text-blue-300 bg-blue-500/10'
                : 'text-blue-400/80 hover:text-blue-300 hover:bg-blue-500/8',
              collapsed ? 'justify-center' : '',
            ].join(' ')}
          >
            <div className="relative shrink-0">
              <Loader2 size={17} className="animate-spin" />
              <span className="absolute -top-1.5 -right-1.5 min-w-[14px] h-3.5 rounded-full bg-blue-500
                text-white text-[9px] font-bold flex items-center justify-center px-0.5 leading-none">
                {pendingCount}
              </span>
            </div>
            {!collapsed && (
              <span className="truncate">
                {pendingCount} job{pendingCount !== 1 ? 's' : ''} running
              </span>
            )}
            {collapsed && (
              <span className="absolute left-full ml-3 px-2 py-1 rounded-md bg-zinc-800 text-white text-xs font-medium
                opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity duration-150 whitespace-nowrap shadow-lg">
                {pendingCount} job{pendingCount !== 1 ? 's' : ''} running
              </span>
            )}
          </NavLink>
        </nav>
      )}

      {/* Bottom nav */}
      <nav className="flex flex-col gap-0.5 px-2 py-3 shrink-0">
        {BOTTOM_NAV.map(({ path, label, icon: Icon }) => (
          <NavLink
            key={path}
            to={path}
            className={({ isActive }) => [
              'group relative flex items-center gap-3 rounded-lg px-3 h-10 text-sm font-medium',
              'transition-all duration-150',
              isActive ? 'text-white bg-white/8' : 'text-white/40 hover:text-white/70 hover:bg-white/5',
              collapsed ? 'justify-center' : '',
            ].join(' ')}
          >
            <Icon size={17} className="shrink-0" strokeWidth={2} />
            {!collapsed && <span className="truncate">{label}</span>}
            {collapsed && (
              <span className="absolute left-full ml-3 px-2 py-1 rounded-md bg-zinc-800 text-white text-xs font-medium
                opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity duration-150 whitespace-nowrap shadow-lg">
                {label}
              </span>
            )}
          </NavLink>
        ))}
      </nav>

      {/* Collapse toggle — hidden on mobile where the backdrop handles closing */}
      <button
        onClick={() => onCollapsedChange(!collapsed)}
        className="hidden md:flex items-center justify-center h-10 mx-2 mb-3 rounded-lg text-white/30 hover:text-white/60 hover:bg-white/5 transition-all duration-150"
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      >
        {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
      </button>
    </aside>
  )
}
