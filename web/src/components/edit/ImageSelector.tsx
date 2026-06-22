import { useRef, useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { Check, ChevronDown, ChevronRight, Upload } from 'lucide-react'
import { fetchProviderImages, setImageFromUrl, uploadImage } from '../../api/images'
import { Skeleton } from '../ui/Skeleton'
import type { ProviderImage } from '../../types'

export type EntityPath = 'library-entries' | 'groups' | 'items' | 'people'

interface ImageSelectorProps {
  entityType: EntityPath
  entityId: string
  currentImageUrl?: string
  onImageSet?: () => void
}

export function sourceBadgeLabel(source: string): string {
  const labels: Record<string, string> = {
    stashdb: 'StashDB',
    fanart: 'Fanart',
    tmdb: 'TMDB',
    tvdb: 'TVDB',
    theaudiodb: 'AudioDB',
    musicbrainz: 'MB',
  }
  return labels[source] ?? source
}

export function ImageSelector({ entityType, entityId, onImageSet }: ImageSelectorProps) {
  const [expanded, setExpanded] = useState(false)
  const [selectedUrl, setSelectedUrl] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const { data: images, isLoading } = useQuery({
    queryKey: ['provider-images', entityType, entityId],
    queryFn: () => fetchProviderImages(entityType, entityId),
    enabled: expanded,
    staleTime: 60_000,
  })

  const setMutation = useMutation({
    mutationFn: (url: string) => setImageFromUrl(entityType, entityId, url),
    onSuccess: (_data, url) => {
      setSelectedUrl(url)
      onImageSet?.()
    },
  })

  const uploadMutation = useMutation({
    mutationFn: (file: File) => uploadImage(entityType, entityId, file),
    onSuccess: () => {
      setSelectedUrl('__upload__')
      onImageSet?.()
    },
  })

  const isBusy = setMutation.isPending || uploadMutation.isPending

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) uploadMutation.mutate(file)
    e.target.value = ''
  }

  return (
    <div className="mt-6 border-t border-white/10 pt-6">
      <button
        type="button"
        onClick={() => setExpanded(v => !v)}
        className="flex w-full items-center gap-2 text-xs font-semibold uppercase tracking-widest text-white/50 hover:text-white/70 transition-colors"
      >
        {expanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
        Images
      </button>

      {expanded && (
        <div className="mt-4">
          {isLoading ? (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="w-full" aspect="16/9" />
              ))}
            </div>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(120px,1fr))] gap-3">
              {images?.map(img => (
                <ProviderThumbnail
                  key={img.url}
                  image={img}
                  selected={selectedUrl === img.url}
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
      )}
    </div>
  )
}

interface ThumbnailProps {
  image: ProviderImage
  selected: boolean
  disabled: boolean
  onClick: () => void
}

function ProviderThumbnail({ image, selected, disabled, onClick }: ThumbnailProps) {
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
    </button>
  )
}
