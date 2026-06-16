import { useQuery } from '@tanstack/react-query'
import { get } from './client'

export interface ModuleConfig {
  enabled: boolean
  roots:   string[]
}

export interface AppConfig {
  server: {
    port: number
  }
  database: {
    driver: string
    dsn: string
  }
  media: {
    path: string
  }
  modules: {
    movies:    ModuleConfig
    tv:        ModuleConfig
    music:     ModuleConfig
    books:     ModuleConfig
    afterdark: ModuleConfig
    jav:       ModuleConfig
  }
  log: {
    level:  string
    format: string
  }
}

export function useAppConfig() {
  return useQuery({
    queryKey: ['config'],
    queryFn: () => get<AppConfig>('/config'),
    staleTime: Infinity,
  })
}
