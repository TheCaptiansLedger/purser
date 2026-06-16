import { Outlet, NavLink } from 'react-router-dom'
import type { LucideIcon } from 'lucide-react'

export interface ModuleTab {
  path: string
  label: string
  icon: LucideIcon
  end?: boolean
}

interface Props {
  tabs: ModuleTab[]
  accent: string
}

export function ModuleLayout({ tabs, accent }: Props) {
  return (
    <div className="relative min-h-screen flex flex-col">
      <div className="flex-1 pb-24">
        <Outlet />
      </div>

      <nav
        className="fixed bottom-0 right-0 z-50 flex items-center justify-around px-4 h-20 border-t"
        style={{
          left: 'var(--sidebar-width, 0px)',
          background: 'rgba(8, 8, 14, 0.88)',
          backdropFilter: 'blur(20px)',
          WebkitBackdropFilter: 'blur(20px)',
          borderColor: 'rgba(255,255,255,0.06)',
        }}
      >
        {tabs.map(({ path, label, icon: Icon, end }) => (
          <NavLink
            key={path}
            to={path}
            end={end}
            className={({ isActive }) => [
              'flex flex-col items-center gap-1 px-4 py-2 rounded-xl transition-all duration-150 min-w-0',
              isActive ? 'text-white' : 'text-white/35 hover:text-white/60',
            ].join(' ')}
          >
            {({ isActive }) => (
              <>
                <div
                  className="relative flex items-center justify-center w-10 h-7 rounded-lg transition-all duration-150"
                  style={isActive ? { background: accent + '22' } : {}}
                >
                  <Icon
                    size={20}
                    strokeWidth={isActive ? 2.5 : 2}
                    style={isActive ? { color: accent } : {}}
                  />
                  {isActive && (
                    <span
                      className="absolute -bottom-1 left-1/2 -translate-x-1/2 w-1 h-1 rounded-full"
                      style={{ background: accent }}
                    />
                  )}
                </div>
                <span
                  className="text-[10px] font-medium tracking-wide truncate"
                  style={isActive ? { color: accent } : {}}
                >
                  {label}
                </span>
              </>
            )}
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
