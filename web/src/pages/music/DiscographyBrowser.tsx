import { X, Disc3, Check, Loader2 } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { fetchArtistDiscography } from '../../api/metadata'
import type { ExternalGroup } from '../../types'

interface DiscographyBrowserProps {
  open: boolean
  artistMbid: string
  importedMbids: Set<string>
  accent: string
  onClose: () => void
  onPick: (album: ExternalGroup) => void
}

export function DiscographyBrowser({
  open,
  artistMbid,
  importedMbids,
  accent,
  onClose,
  onPick,
}: DiscographyBrowserProps) {
  const { data, isLoading, isError } = useQuery({
    queryKey: ['discography', 'mbz', artistMbid],
    queryFn: () => fetchArtistDiscography('mbz', 'music', artistMbid, 1, 200),
    enabled: open && !!artistMbid,
    staleTime: 5 * 60 * 1000,
  })

  if (!open) return null

  const albums = data?.results ?? []

  function handlePick(album: ExternalGroup) {
    onPick(album)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-60 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.6)' }}>
      <div className="w-full max-w-md rounded-xl border border-white/10 shadow-2xl flex flex-col" style={{ background: '#0f0f17', maxHeight: '75vh' }}>

        <div className="flex items-center justify-between px-5 py-4 border-b border-white/8">
          <h2 className="text-sm font-semibold text-white">Browse Discography</h2>
          <button onClick={onClose} className="text-white/40 hover:text-white/70 transition-colors">
            <X size={16} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-5 py-4 min-h-0">
          <p className="text-xs text-white/40 mb-3">
            Select a release to pre-fill the form. Already-added releases are greyed out.
          </p>

          {isLoading && (
            <div className="flex items-center justify-center gap-3 py-10 text-white/50">
              <Loader2 size={18} className="animate-spin" />
              <span className="text-sm">Loading discography…</span>
            </div>
          )}

          {isError && (
            <p className="text-sm text-red-400 text-center py-6">Failed to load discography.</p>
          )}

          {!isLoading && !isError && albums.length === 0 && (
            <p className="text-sm text-white/40 text-center py-6">No releases found.</p>
          )}

          {!isLoading && !isError && albums.length > 0 && (
            <div className="space-y-1">
              {albums.map(album => {
                const alreadyImported = importedMbids.has(album.externalId)
                return (
                  <button
                    key={`${album.source}-${album.externalId}`}
                    onClick={() => !alreadyImported && handlePick(album)}
                    disabled={alreadyImported}
                    className="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 disabled:cursor-default transition-colors text-left group"
                  >
                    <div className="w-8 h-8 rounded-md shrink-0 bg-white/5 flex items-center justify-center">
                      {alreadyImported
                        ? <Check size={14} style={{ color: accent }} />
                        : <Disc3 size={14} className="text-white/20" />
                      }
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className={`text-sm font-medium truncate ${alreadyImported ? 'text-white/30' : 'text-white'}`}>
                        {album.title}
                      </p>
                      {album.year ? <p className="text-xs text-white/25">{album.year}</p> : null}
                    </div>
                    {alreadyImported && (
                      <span className="text-xs text-white/25 shrink-0">Added</span>
                    )}
                  </button>
                )
              })}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
