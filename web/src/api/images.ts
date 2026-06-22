import { get } from './client'
import type { ProviderImage } from '../types'

const BASE = '/api/v1'

export function fetchProviderImages(entityPath: string, id: string): Promise<ProviderImage[]> {
  return get<ProviderImage[]>(`/${entityPath}/${id}/provider-images`)
}

export async function setImageFromUrl(entityPath: string, id: string, url: string): Promise<void> {
  const res = await fetch(`${BASE}/${entityPath}/${id}/image`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
}

export async function uploadImage(entityPath: string, id: string, file: File): Promise<void> {
  const form = new FormData()
  form.append('image', file)
  const res = await fetch(`${BASE}/${entityPath}/${id}/image`, {
    method: 'POST',
    body: form,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
}
