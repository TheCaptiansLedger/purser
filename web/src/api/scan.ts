import { useQuery } from '@tanstack/react-query'
import { get, post } from './client'
import type { Job, MatchCandidate, UnmatchedFile, UnmatchedFileGroup, UnmatchedListResponse } from '../types'

interface UnmatchedParams {
  status?: string
  contentType?: string
  groupBy?: string
}

export function useUnmatchedFiles(params: UnmatchedParams = {}) {
  const isPending = !params.status || params.status === 'pending'
  return useQuery({
    queryKey: ['unmatched-files', params],
    queryFn: () =>
      get<UnmatchedListResponse<UnmatchedFile | UnmatchedFileGroup>>('/unmatched-files', {
        status: params.status,
        contentType: params.contentType,
        groupBy: params.groupBy,
      }),
    refetchInterval: isPending ? 30_000 : false,
  })
}

export function useUnmatchedCount() {
  return useQuery({
    queryKey: ['unmatched-files', { status: 'pending', _count: true }],
    queryFn: () => get<UnmatchedListResponse<UnmatchedFile>>('/unmatched-files', { status: 'pending' }),
    refetchInterval: 30_000,
    select: (data) => data.total,
  })
}

// Returns the set of content type strings that have at least one pending file.
// Used by ImportQueuePage to only show tabs for content types present in the queue.
export function usePendingContentTypes() {
  return useQuery({
    queryKey: ['unmatched-files-content-types'],
    queryFn: () => get<UnmatchedListResponse<UnmatchedFile>>('/unmatched-files', { status: 'pending' }),
    refetchInterval: 30_000,
    select: (data) => new Set<string>((data.items as UnmatchedFile[]).map(f => f.content_type)),
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
