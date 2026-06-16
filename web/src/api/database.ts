import { useQuery } from '@tanstack/react-query'

export interface TableStat {
  name: string
  rows: number
}

export interface DBStats {
  tables: TableStat[]
  file_size_bytes: number
  sqlite_version: string
  migration_count: number
}

export interface RestoreResult {
  message: string
  tables: TableStat[]
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
      let body: any
      try { body = JSON.parse(xhr.responseText) } catch { body = {} }
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve(body as RestoreResult)
      } else {
        reject(new Error(body.error ?? xhr.statusText))
      }
    }

    xhr.onerror  = () => reject(new Error('Network error during upload'))
    xhr.ontimeout = () => reject(new Error('Upload timed out'))

    xhr.open('POST', '/api/v1/database/restore')
    xhr.send(form)
  })
}
