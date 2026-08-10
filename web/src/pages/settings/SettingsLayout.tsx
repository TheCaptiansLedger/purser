import { NavLink, Outlet } from 'react-router-dom'

// TAB_ITEMS is a registry, not a hardcoded tab bar — mirrors the
// NAV_ITEMS pattern in components/layout/Sidebar.tsx. Settings is the
// only tabbed section today, so this stays local to the page rather than
// a generic components/Tabs.tsx; extract one if a second module needs
// tabs later.
interface TabItem {
  to: string
  label: string
}

const TAB_ITEMS: TabItem[] = [
  { to: '/settings/config', label: 'Config' },
  { to: '/settings/jobs', label: 'Jobs' },
  { to: '/settings/database', label: 'Database' },
  { to: '/settings/cache', label: 'Cache' },
]

// SettingsLayout is the tab shell every /settings/* route renders inside
// — see docs/adr/0028-layered-settings.md and #596. It owns only the tab
// bar and the outlet; each tab's actual content (Config #604, Jobs
// #606-608, Database #612-613, Cache #614-617) is a separate route
// component.
export function SettingsLayout() {
  return (
    <div className="px-6 py-10 md:px-8">
      <h1 className="text-headline text-text">Settings</h1>

      <nav className="mt-6 flex gap-1 border-b border-border" aria-label="Settings tabs">
        {TAB_ITEMS.map(item => (
          <NavLink
            key={item.to}
            to={item.to}
            className={({ isActive }) =>
              [
                'px-3 h-10 flex items-center text-body border-b-2 -mb-px transition-colors',
                'text-text-secondary hover:text-text',
                isActive ? 'text-text border-text' : 'border-transparent',
              ].join(' ')
            }
          >
            {item.label}
          </NavLink>
        ))}
      </nav>

      <div className="mt-6">
        <Outlet />
      </div>
    </div>
  )
}
