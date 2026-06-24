import { useMutation, useQuery } from '@tanstack/react-query'
import { get, patchEmpty } from './client'
import type { ContentTypeConfig, KindConfig } from '../types'

export interface ModuleConfig {
  enabled: boolean
  roots:   string[]
}

export interface SourceConfig {
  enabled:    boolean
  url:        string
  api_key:    string
  user_agent: string
}

export interface AppConfig {
  server: {
    port:    number
    workers: number
  }
  database: {
    driver: string
    dsn:    string
  }
  media: {
    path: string
  }
  modules: Record<string, ModuleConfig>
  sources: Record<string, SourceConfig>
  log: {
    level:  string
    format: string
  }
  locked: Record<string, boolean>
}

export interface PatchConfigRequest {
  modules?: Record<string, { enabled?: boolean; roots?: string[] }>
  sources?: Record<string, { enabled?: boolean; url?: string; api_key?: string; user_agent?: string }>
}

export function useContentTypeConfigs() {
  return useQuery({
    queryKey: ['config', 'content-types'],
    queryFn:  () => get<ContentTypeConfig[]>('/config/content-types'),
    staleTime: Infinity,
  })
}

export function useKindConfigs() {
  return useQuery({
    queryKey: ['config', 'kinds'],
    queryFn:  () => get<KindConfig[]>('/config/kinds'),
    staleTime: Infinity,
  })
}

export function useAppConfig() {
  return useQuery({
    queryKey: ['config'],
    queryFn: () => get<AppConfig>('/config'),
    staleTime: Infinity,
  })
}

export function usePatchConfig() {
  return useMutation({
    mutationFn: (body: PatchConfigRequest) => patchEmpty('/config', body),
  })
}
