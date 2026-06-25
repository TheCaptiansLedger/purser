import { Film, Tv2, Music2, BookOpen, Sparkles, Video } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { EnabledModules } from '../context/ModulesContext'

export interface ModuleRegistryEntry {
  key:    keyof EnabledModules
  label:  string
  path:   string
  icon:   LucideIcon
  accent: string
}

export const MODULE_REGISTRY: ModuleRegistryEntry[] = [
  { key: 'movies',    label: 'Movies',    path: '/movies',    icon: Film,     accent: '#3b82f6' },
  { key: 'tv',        label: 'TV Shows',  path: '/tv',        icon: Tv2,      accent: '#8b5cf6' },
  { key: 'music',     label: 'Music',     path: '/music',     icon: Music2,   accent: '#10b981' },
  { key: 'books',     label: 'Books',     path: '/books',     icon: BookOpen, accent: '#f59e0b' },
  { key: 'afterdark', label: 'AfterDark', path: '/afterdark', icon: Sparkles, accent: '#f43f5e' },
  { key: 'jav',       label: 'JAV',       path: '/jav',       icon: Video,    accent: '#ec4899' },
]
