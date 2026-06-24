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
  modules: {
    movies:    ModuleConfig
    tv:        ModuleConfig
    music:     ModuleConfig
    books:     ModuleConfig
    afterdark: ModuleConfig
    jav:       ModuleConfig
  }
  sources: {
    stashdb:     SourceConfig
    tpdb:        SourceConfig
    stash:       SourceConfig
    tmdb:        SourceConfig
    tvdb:        SourceConfig
    musicbrainz: SourceConfig
    fanart:      SourceConfig
    lastfm:      SourceConfig
    theaudiodb:  SourceConfig
    openlibrary: SourceConfig
  }
  log: {
    level:  string
    format: string
  }
  locked: Record<string, boolean>
}

export interface PatchConfigRequest {
  modules?: {
    movies?:    { enabled?: boolean; roots?: string[] }
    tv?:        { enabled?: boolean; roots?: string[] }
    music?:     { enabled?: boolean; roots?: string[] }
    books?:     { enabled?: boolean; roots?: string[] }
    afterdark?: { enabled?: boolean; roots?: string[] }
  }
  sources?: {
    tmdb?:        { api_key?: string }
    tvdb?:        { api_key?: string }
    musicbrainz?: { api_key?: string }
    fanart?:      { api_key?: string }
    theaudiodb?:  { api_key?: string }
    stashdb?:     { api_key?: string; url?: string }
  }
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
