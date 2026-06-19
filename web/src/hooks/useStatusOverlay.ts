import { useState, useCallback } from 'react'

const storageKey = (module: string) => `purser:statusOverlay:${module}`

export function useStatusOverlay(module: string): [boolean, () => void] {
  const key = storageKey(module)
  const [alwaysShow, setAlwaysShow] = useState<boolean>(() => {
    try {
      return localStorage.getItem(key) === 'true'
    } catch {
      return false
    }
  })

  const toggle = useCallback(() => {
    setAlwaysShow(prev => {
      const next = !prev
      try {
        localStorage.setItem(key, String(next))
      } catch {
        // storage unavailable — state still updates in memory
      }
      return next
    })
  }, [key])

  return [alwaysShow, toggle]
}
