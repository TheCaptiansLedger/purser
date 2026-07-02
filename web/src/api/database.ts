import { useQuery } from '@tanstack/react-query'

export interface CollectionStat {
  name: string
  count: number
}

export interface DBStats {
  driver: string
  driver_version: string
  size_bytes: number
  collections: CollectionStat[]
  extra?: Record<string, unknown>
}

export interface RestoreResult {
  message: string
  collections: CollectionStat[]
  total_rows: number
}

export function useDBStats() {
  return useQuery({
    queryKey: ['db-stats'],
    queryFn: async () => {
      const res = await fetch('/api/v1/database/stats')
      if (!res.ok) throw new Error('failed to fetch database stats')
      return res.json() as Promise<DBStats>
    },
  })
}

export function uploadWithProgress(
  file: File,
  onProgress: (pct: number) => void,
): Promise<RestoreResult> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    const form = new FormData()
    form.append('database', file)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) {
        onProgress(Math.min(99, Math.round((e.loaded / e.total) * 100)))
      }
    }

    xhr.onload = () => {
      let body: unknown
      try { body = JSON.parse(xhr.responseText) } catch { body = {} }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(body as RestoreResult)
      } else {
        const err = (body as Record<string, string>).error ?? xhr.statusText
        reject(new Error(err))
      }
    }

    xhr.onerror  = () => reject(new Error('Network error during upload'))
    xhr.ontimeout = () => reject(new Error('Upload timed out'))

    xhr.open('POST', '/api/v1/database/restore')
    xhr.send(form)
  })
}
