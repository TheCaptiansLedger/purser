import { useQuery } from '@tanstack/react-query'
import { get, post } from './client'
import type { Job, MatchCandidate, UnmatchedFile, UnmatchedFileGroup, UnmatchedListResponse } from '../types'

interface UnmatchedParams {
  status?: string
  contentType?: string
  groupBy?: string
}

export function useUnmatchedFiles(params: UnmatchedParams = {}) {
  return useQuery({
    queryKey: ['unmatched-files', params],
    queryFn: () =>
      get<UnmatchedListResponse<UnmatchedFile | UnmatchedFileGroup>>('/unmatched-files', {
        status: params.status,
        contentType: params.contentType,
        groupBy: params.groupBy,
      }),
  })
}

export function useUnmatchedFile(id: string) {
  return useQuery({
    queryKey: ['unmatched-files', id],
    queryFn: () => get<UnmatchedFile>(`/unmatched-files/${id}`),
    enabled: !!id,
  })
}

export function manualMatch(id: string, itemId: string): Promise<UnmatchedFile> {
  return post<UnmatchedFile>(`/unmatched-files/${id}/match`, { item_id: itemId })
}

interface RescrapeResponse {
  candidates: MatchCandidate[]
}

export function rescrape(id: string, query?: string): Promise<RescrapeResponse> {
  return post<RescrapeResponse>(`/unmatched-files/${id}/scrape`, { query: query ?? '' })
}

export function dismissUnmatched(id: string): Promise<void> {
  return post<void>(`/unmatched-files/${id}/dismiss`, {})
}

export function triggerScan(entryId?: string): Promise<Job> {
  if (entryId) {
    return post<Job>('/commands', { name: 'ScanLibrary', entryId })
  }
  return post<Job>('/commands', { name: 'ScanAllRoots' })
}
