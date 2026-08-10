import { ChevronLeft, ChevronRight, Home, Settings } from 'lucide-react'
import { NavLink } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'

// NAV_ITEMS is a registry, not a hardcoded single link — module pages
// (Music, AfterDark, Acquisition, Pipeline/Jobs) each add an entry here
// later, per docs/design/ux-principles.md's "persistent primary
// navigation across modules" rule.
//
// end controls NavLink's exact-match behavior: '/' must use exact
// matching (every path starts with '/'), but a sectioned entry like
// '/settings' should stay highlighted on its sub-routes
// (/settings/config, /settings/jobs, ...), so it's computed per item
// below rather than hardcoded.
interface NavItem {
  to: string
  label: string
  icon: LucideIcon
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Welcome', icon: Home },
  { to: '/settings', label: 'Settings', icon: Settings },
]

interface SidebarProps {
  collapsed: boolean
  onCollapsedChange: (collapsed: boolean) => void
  mobileOpen?: boolean
}

// Sidebar is the app's one fixed-width layout element — see
// docs/design/style-guide.md#fluid-by-default--no-centered-container.
// Below the md breakpoint it becomes an off-canvas drawer (translate-x),
// per docs/design/frontend-stack.md#styling-tailwind-v4.
export function Sidebar({ collapsed, onCollapsedChange, mobileOpen }: SidebarProps) {
  return (
    <aside
      className={[
        'fixed inset-y-0 left-0 z-40 flex flex-col',
        'bg-surface border-r border-border',
        'transition-all duration-300 ease-in-out',
        collapsed ? 'w-16' : 'w-60',
        mobileOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0',
      ].join(' ')}
    >
      <div className="flex items-center h-14 px-4 shrink-0">
        <span className="text-title-md font-semibold text-text truncate">
          {collapsed ? 'P' : 'Purser'}
        </span>
      </div>

      <nav className="flex-1 px-2 py-2 flex flex-col gap-1" aria-label="Primary">
        {NAV_ITEMS.map(item => {
          const Icon = item.icon
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                [
                  'flex items-center gap-3 h-10 px-3 rounded-lg text-body',
                  'text-text-secondary hover:text-text hover:bg-surface-raised transition-colors',
                  isActive ? 'text-text bg-surface-raised' : '',
                ].join(' ')
              }
            >
              <Icon size={16} className="shrink-0" />
              {!collapsed && <span className="truncate">{item.label}</span>}
            </NavLink>
          )
        })}
      </nav>

      <button
        onClick={() => onCollapsedChange(!collapsed)}
        className="hidden md:flex items-center justify-center h-10 mx-2 mb-3 rounded-lg text-text-muted hover:text-text-secondary hover:bg-surface-raised transition-colors"
        aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
      >
        {collapsed ? <ChevronRight size={16} /> : <ChevronLeft size={16} />}
      </button>
    </aside>
  )
}
