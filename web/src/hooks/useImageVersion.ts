import { useState, useCallback } from 'react'

export function buildVersionedUrl(baseUrl: string | undefined, version: number): string | undefined {
  return baseUrl ? `${baseUrl}?v=${version}` : undefined
}

export function useImageVersion(baseUrl: string | undefined): [string | undefined, () => void] {
  const [version, setVersion] = useState(0)
  const bumpVersion = useCallback(() => setVersion(v => v + 1), [])
  return [buildVersionedUrl(baseUrl, version), bumpVersion]
}
