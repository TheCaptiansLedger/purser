import { createContext, useContext } from 'react'
import type { ReactNode } from 'react'
import { useAppConfig } from '../api/config'

export interface EnabledModules {
  movies:    boolean
  tv:        boolean
  music:     boolean
  books:     boolean
  afterdark: boolean
  jav:       boolean
}

const ALL_ENABLED: EnabledModules = {
  movies: true, tv: true, music: true, books: true, afterdark: true, jav: true,
}

const ModulesContext = createContext<EnabledModules>(ALL_ENABLED)

export function ModulesProvider({ children }: { children: ReactNode }) {
  const { data } = useAppConfig()
  const modules: EnabledModules = data
    ? {
        movies:    data.modules.movies.enabled,
        tv:        data.modules.tv.enabled,
        music:     data.modules.music.enabled,
        books:     data.modules.books.enabled,
        afterdark: data.modules.afterdark.enabled,
        jav:       data.modules.jav.enabled,
      }
    : ALL_ENABLED

  return (
    <ModulesContext.Provider value={modules}>
      {children}
    </ModulesContext.Provider>
  )
}

export function useModules(): EnabledModules {
  return useContext(ModulesContext)
}
