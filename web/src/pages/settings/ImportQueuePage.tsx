import { useState, useMemo } from 'react'
import { ScanLine, Inbox } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useUnmatchedFiles, usePendingContentTypes, triggerScan } from '../../api/scan'
import { useContentTypeConfigs, useAppConfig } from '../../api/config'
import { ChipTabs } from '../../components/ui/ChipTabs'
import type { ChipTab } from '../../components/ui/ChipTabs'
import { UnmatchedFileCard } from '../../components/scan/UnmatchedFileCard'
import { UnmatchedFileDetail } from '../../components/scan/UnmatchedFileDetail'
import { AlbumUnmatchedGroup } from '../../components/scan/AlbumUnmatchedGroup'
import type { UnmatchedFile, UnmatchedFileGroup, ContentType } from '../../types'

type StatusFilter = 'pending' | 'dismissed'

export function ImportQueuePage() {
  const qc = useQueryClient()

  const { data: contentTypeConfigs = [] } = useContentTypeConfigs()
  const { data: appConfig } = useAppConfig()
  const { data: pendingTypes } = usePendingContentTypes()

  const enabledModules = appConfig?.modules ?? {}

  // Build tabs: only content types that are (a) module-enabled AND (b) have pending files.
  // While pendingTypes is loading, show all enabled content types so the UI doesn't flash empty.
  const tabs: ChipTab<string>[] = useMemo(() => {
    return contentTypeConfigs
      .filter(ct => enabledModules[ct.moduleKey]?.enabled)
      .filter(ct => !pendingTypes || pendingTypes.has(ct.contentType))
      .map(ct => ({ id: ct.contentType, label: ct.label }))
  }, [contentTypeConfigs, enabledModules, pendingTypes])

  const [activeTab, setActiveTab]         = useState<string>('')
  const [status, setStatus]               = useState<StatusFilter>('pending')
  const [groupByAlbum, setGroupByAlbum]   = useState(false)
  const [selectedFileId, setSelectedFileId] = useState<string | null>(null)
  const [scanning, setScanning]           = useState(false)

  // Default to first tab when tabs load
  const effectiveTab = activeTab || tabs[0]?.id || ''

  const activeConfig   = contentTypeConfigs.find(ct => ct.contentType === effectiveTab)
  const supportsGrouping = activeConfig?.supportsFileGrouping ?? false
  const groupBy        = groupByAlbum && supportsGrouping ? 'album' : undefined

  const { data, isLoading, refetch } = useUnmatchedFiles({
    status,
    contentType: effectiveTab as ContentType || undefined,
    groupBy,
  })

  const items   = data?.items ?? []
  const total   = data?.total ?? 0
  const grouped = data?.grouped_by === 'album'

  const handleResolved = () => {
    setSelectedFileId(null)
    void refetch()
    qc.invalidateQueries({ queryKey: ['unmatched-files'] })
    qc.invalidateQueries({ queryKey: ['unmatched-files-content-types'] })
  }

  const handleScanNow = async () => {
    setScanning(true)
    try { await triggerScan() }
    finally { setScanning(false) }
  }

  const handleTabChange = (id: string) => {
    setActiveTab(id)
    setGroupByAlbum(false)
    setSelectedFileId(null)
  }

  const flatFiles    = !grouped ? (items as UnmatchedFile[]) : []
  const selectedFile = flatFiles.find(f => f.id === selectedFileId) ?? null

  const isEmpty = !isLoading && items.length === 0

  return (
    <div className="flex flex-col" style={{ height: 'calc(100vh - 120px)' }}>
      {/* Header */}
      <div className="px-8 pt-8 pb-4 shrink-0">
        <h1 className="text-2xl font-semibold text-white">Import Queue</h1>
        <p className="text-white/40 text-sm mt-1">Files identified on disk that need your attention</p>
      </div>

      {tabs.length === 0 && !pendingTypes ? (
        <div className="px-8">
          <p className="text-sm text-white/30">No modules are enabled.</p>
        </div>
      ) : tabs.length === 0 ? (
        /* Pending types loaded and queue is empty across all modules */
        <div className="flex flex-col items-center justify-center flex-1 text-center px-8">
          <Inbox size={40} className="text-white/15 mb-4" strokeWidth={1} />
          <p className="text-white/50 font-medium">Queue is empty</p>
          <p className="text-white/25 text-sm mt-1 mb-6">Run a scan to discover files on disk.</p>
          <button
            onClick={() => { void handleScanNow() }}
            disabled={scanning}
            className="flex items-center gap-2 text-sm font-medium px-4 py-2 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 disabled:opacity-40 transition-colors"
          >
            <ScanLine size={14} className={scanning ? 'animate-pulse' : ''} />
            {scanning ? 'Starting scan…' : 'Scan Now'}
          </button>
        </div>
      ) : (
        <>
          {/* Filter bar */}
          <div className="px-8 shrink-0">
            <ChipTabs
              tabs={tabs}
              value={effectiveTab}
              onChange={handleTabChange}
              accent="#6366f1"
              rightControls={
                <div className="flex items-center gap-3">
                  {supportsGrouping && !grouped && (
                    <label className="flex items-center gap-2 text-xs text-white/40 cursor-pointer select-none">
                      <input
                        type="checkbox"
                        checked={groupByAlbum}
                        onChange={e => setGroupByAlbum(e.target.checked)}
                        className="accent-indigo-500"
                      />
                      Group by album
                    </label>
                  )}
                  <div className="flex items-center gap-1">
                    {(['pending', 'dismissed'] as StatusFilter[]).map(s => (
                      <button
                        key={s}
                        onClick={() => setStatus(s)}
                        className={[
                          'text-xs px-2.5 py-1 rounded-lg transition-colors capitalize',
                          status === s
                            ? 'bg-white/10 text-white/80'
                            : 'text-white/30 hover:text-white/60',
                        ].join(' ')}
                      >
                        {s}
                      </button>
                    ))}
                  </div>
                </div>
              }
            />
          </div>

          {/* Content */}
          {isLoading ? (
            <div className="px-8 mt-4 space-y-3">
              {[1, 2, 3].map(i => (
                <div key={i} className="h-16 rounded-xl bg-white/3 animate-pulse" />
              ))}
            </div>
          ) : isEmpty ? (
            <div className="flex flex-col items-center justify-center flex-1 text-center px-8">
              <ScanLine size={40} className="text-white/15 mb-4" strokeWidth={1} />
              <p className="text-white/50 font-medium">No files need attention</p>
              <p className="text-white/25 text-sm mt-1 mb-6">
                {status === 'pending'
                  ? 'Run a scan to discover files on disk.'
                  : 'No dismissed files for this content type.'}
              </p>
              {status === 'pending' && (
                <button
                  onClick={() => { void handleScanNow() }}
                  disabled={scanning}
                  className="flex items-center gap-2 text-sm font-medium px-4 py-2 rounded-lg border border-white/10 text-white/50 hover:text-white/80 hover:border-white/20 disabled:opacity-40 transition-colors"
                >
                  <ScanLine size={14} className={scanning ? 'animate-pulse' : ''} />
                  {scanning ? 'Starting scan…' : 'Scan Now'}
                </button>
              )}
            </div>
          ) : grouped ? (
            /* Grouped (album) view — full width */
            <div className="px-8 mt-4 overflow-y-auto flex-1 space-y-3 pb-8">
              <p className="text-xs text-white/30 mb-2">
                {total} album group{total !== 1 ? 's' : ''}
              </p>
              {(items as UnmatchedFileGroup[]).map(g => (
                <AlbumUnmatchedGroup key={g.group_id} group={g} onResolved={handleResolved} />
              ))}
            </div>
          ) : (
            /* Flat view — master-detail split */
            <div className="flex flex-1 overflow-hidden mt-2 min-h-0">
              {/* Left: file list */}
              <div className="flex flex-col border-r border-white/5 overflow-hidden" style={{ width: '38%', minWidth: '240px' }}>
                <p className="text-[10px] text-white/25 px-4 py-2 shrink-0 border-b border-white/5">
                  {total} file{total !== 1 ? 's' : ''}
                </p>
                <div className="overflow-y-auto flex-1">
                  {flatFiles.map(f => (
                    <UnmatchedFileCard
                      key={f.id}
                      file={f}
                      selected={selectedFileId === f.id}
                      onSelect={() => setSelectedFileId(f.id)}
                    />
                  ))}
                </div>
              </div>

              {/* Right: detail panel */}
              <div className="flex-1 overflow-y-auto">
                {selectedFile ? (
                  <UnmatchedFileDetail file={selectedFile} onResolved={handleResolved} />
                ) : (
                  <div className="flex flex-col items-center justify-center h-full text-center px-8">
                    <Inbox size={32} className="text-white/10 mb-3" strokeWidth={1} />
                    <p className="text-sm text-white/25">Select a file to review details</p>
                  </div>
                )}
              </div>
            </div>
          )}
        </>
      )}
    </div>
  )
}
