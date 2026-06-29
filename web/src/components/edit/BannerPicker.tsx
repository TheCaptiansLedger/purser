import { useRef } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Check, Trash2, Upload, X } from 'lucide-react'
import { fetchProviderImages, setEntryBannerFromUrl, uploadEntryBanner, clearEntryBanner } from '../../api/images'
import { Skeleton } from '../ui/Skeleton'
import { sourceBadgeLabel } from './ImageSelector'
import type { ProviderImage } from '../../types'

interface BannerPickerProps {
  entryId: string
  currentBannerUrl?: string
  onClose: () => void
  onBannerSet: () => void
}

export function BannerPicker({ entryId, currentBannerUrl, onClose, onBannerSet }: BannerPickerProps) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: images, isLoading } = useQuery({
    queryKey: ['provider-images', 'library-entries', entryId],
    queryFn: () => fetchProviderImages('library-entries', entryId),
    staleTime: 60_000,
  })

  const setMutation = useMutation({
    mutationFn: (url: string) => setEntryBannerFromUrl(entryId, url),
    onSuccess: () => { onBannerSet(); onClose() },
  })

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadEntryBanner(entryId, file),
    onSuccess: () => { onBannerSet(); onClose() },
  })

  const clearMutation = useMutation({
    mutationFn: () => clearEntryBanner(entryId),
    onSuccess: () => { onBannerSet(); onClose() },
  })

  const isBusy = setMutation.isPending || uploadMutation.isPending || clearMutation.isPending

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) uploadMutation.mutate(file)
    e.target.value = ''
  }

  return (
    <>
      <div className="fixed inset-0 z-40 bg-black/60" onClick={onClose} />
      <div className="fixed inset-y-0 right-0 z-50 flex w-[52vw] flex-col border-l border-white/10 bg-zinc-900/95 shadow-2xl backdrop-blur-xl">
        <div className="flex items-center gap-4 border-b border-white/10 px-8 py-5">
          <h2 className="flex-1 text-lg font-semibold text-white">Banner Image</h2>
          {currentBannerUrl && (
            <button
              type="button"
              onClick={() => clearMutation.mutate()}
              disabled={isBusy}
              className="flex items-center gap-1.5 rounded-lg border border-white/10 px-3 py-1.5 text-xs text-white/50 transition-colors hover:border-red-500/40 hover:text-red-400 disabled:opacity-50"
            >
              <Trash2 size={12} />
              Remove
            </button>
          )}
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-white/40 transition-colors hover:text-white/80"
          >
            <X size={20} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-8 py-6">
          <p className="mb-4 text-xs text-white/40">
            Select a wide image to use as the banner backdrop. Landscape images work best.
          </p>

          {isLoading ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="w-full" aspect="16/9" />
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-3">
              {images?.map(img => (
                <BannerThumbnail
                  key={img.url}
                  image={img}
                  selected={img.url === currentBannerUrl}
                  disabled={isBusy}
                  onClick={() => setMutation.mutate(img.url)}
                />
              ))}
              <button
                type="button"
                disabled={isBusy}
                onClick={() => fileInputRef.current?.click()}
                className="relative flex aspect-video flex-col items-center justify-center gap-1.5 rounded-lg border border-dashed border-white/20 bg-white/5 text-white/40 transition-colors hover:border-white/40 hover:text-white/60 disabled:opacity-50"
              >
                <Upload size={16} />
                <span className="text-[10px] font-medium uppercase tracking-wide">Upload</span>
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/jpeg,image/png,image/gif,image/webp"
                className="hidden"
                onChange={handleFileChange}
              />
            </div>
          )}
        </div>
      </div>
    </>
  )
}

interface BannerThumbnailProps {
  image: ProviderImage
  selected: boolean
  disabled: boolean
  onClick: () => void
}

function BannerThumbnail({ image, selected, disabled, onClick }: BannerThumbnailProps) {
  return (
    <button
      type="button"
      disabled={disabled}
      onClick={onClick}
      className={[
        'relative aspect-video overflow-hidden rounded-lg border transition-all',
        selected ? 'border-white/60 ring-2 ring-white/60' : 'border-white/10 hover:border-white/30',
        disabled ? 'cursor-default opacity-50' : '',
      ].join(' ')}
    >
      <img src={image.url} alt="" className="h-full w-full object-cover" loading="lazy" />
      {selected && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/30">
          <Check size={20} className="text-white drop-shadow" />
        </div>
      )}
      <span className="absolute bottom-1 left-1 rounded bg-black/60 px-1.5 py-0.5 text-[9px] font-medium uppercase tracking-wide text-white/80">
        {sourceBadgeLabel(image.source)}
      </span>
      {image.width > 0 && image.height > 0 && (
        <span className="absolute bottom-1 right-1 rounded bg-black/60 px-1.5 py-0.5 text-[9px] text-white/60">
          {image.width}×{image.height}
        </span>
      )}
    </button>
  )
}
