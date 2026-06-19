import { post } from './client'
import type { Job } from '../types'

export function refreshStudio(entryId: string) {
  return post<Job>('/commands', { name: 'RefreshStudio', entryId })
}

export function refreshArtist(entryId: string) {
  return post<Job>('/commands', { name: 'RefreshArtist', entryId })
}
