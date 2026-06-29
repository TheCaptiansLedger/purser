import { useState } from 'react'
import { Pencil } from 'lucide-react'
import { useQueryClient } from '@tanstack/react-query'
import { useImageVersion } from '../../hooks/useImageVersion'
import { BannerPicker } from '../edit/BannerPicker'
import { Hero } from './Hero'
import type { LibraryEntry } from '../../types'

interface Props {
  entry: LibraryEntry
  backdropFallbackUrl?: string
  accent?: string
  children: React.ReactNode
}

export function resolveHeroBackdrop(
  bannerUrl: string | undefined,
  fallbackUrl: string | undefined,
): { url: string | undefined; blur: boolean } {
  return {
    url: bannerUrl ?? fallbackUrl,
    blur: !bannerUrl,
  }
}

export function EntryHero({ entry, backdropFallbackUrl, accent, children }: Props) {
  const queryClient = useQueryClient()
  const [bannerPickerOpen, setBannerPickerOpen] = useState(false)
  const [versionedBannerUrl, bumpBannerVersion] = useImageVersion(entry.bannerUrl)
  const { url, blur } = resolveHeroBackdrop(versionedBannerUrl, backdropFallbackUrl)

  return (
    <div className="group relative">
      <Hero backdropUrl={url} backdropBlur={blur} accent={accent}>
        {children}
      </Hero>
      <div className="absolute top-3 right-3 opacity-0 group-hover:opacity-100 transition-opacity z-20">
        <button
          type="button"
          onClick={() => setBannerPickerOpen(true)}
          aria-label="Change banner image"
          className="flex items-center gap-1.5 rounded-lg border border-white/20 bg-black/50 px-3 py-1.5 text-xs text-white/70 backdrop-blur-sm transition-colors hover:border-white/40 hover:text-white"
        >
          <Pencil size={12} />
          Banner
        </button>
      </div>
      {bannerPickerOpen && (
        <BannerPicker
          entryId={entry.id}
          currentBannerUrl={entry.bannerUrl}
          onClose={() => setBannerPickerOpen(false)}
          onBannerSet={() => {
            bumpBannerVersion()
            queryClient.invalidateQueries({ queryKey: ['library-entries', entry.id] })
          }}
        />
      )}
    </div>
  )
}
