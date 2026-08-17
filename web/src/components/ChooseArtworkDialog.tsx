import { Loader2, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import type { Image } from '../gen/purser/domain/v1/image_pb'
import { useAttachImage } from '../hooks/useAttachImage'
import { readAsArrayBuffer } from '../lib/readAsArrayBuffer'
import { Modal } from './Modal'

export interface ArtworkCandidate {
  // Hotlinked provider URL, rendered directly (never re-fetched through
  // our own store just for the picker) — the same URL the calling
  // screen's Lookup RPC result already showed.
  url: string
  // Recorded on the created Image row's source field, e.g. "fanart.tv",
  // "theaudiodb" — the caller knows which provider a candidate came from,
  // this dialog doesn't.
  source: string
  label?: string
}

export interface ChooseArtworkDialogProps {
  title: string
  ownerType: string
  ownerId: string
  imageType: string
  priority?: number
  // Provider candidates already fetched by the caller (e.g. from
  // FanartTVService.LookupArtist) — omit/empty for an upload-only flow.
  candidates?: ArtworkCandidate[]
  onClose: () => void
  onAttached: (image: Image) => void
}

// ChooseArtworkDialog is #656's choose-artwork UI flow: the two entry
// points docs/technical/image-caching-and-serving.md names — pick a
// provider image already shown from a Lookup RPC result, or upload a
// local file — built on useAttachImage so this is the one place either
// flow's call order lives. Generic over ownerType/ownerId/imageType so
// every consuming screen (Person photo, Artist poster, Album cover, ...)
// reuses it unmodified, per ADR 0004's Component-First rule.
export function ChooseArtworkDialog({
  title,
  ownerType,
  ownerId,
  imageType,
  priority,
  candidates = [],
  onClose,
  onAttached,
}: ChooseArtworkDialogProps) {
  const { attachFromUrl, attachFromFile, cacheRemoteImage, uploadImage, createImage, selectImage } = useAttachImage()
  const [pendingUrl, setPendingUrl] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Both entry points converge on CreateImage/SelectImage, so their
  // pending/error state is shared; the blob-call state stays split by
  // which entry point is in flight (see useAttachImage's own reasoning
  // for keeping these apart instead of collapsing them into one status).
  const blobError = cacheRemoteImage.error ?? uploadImage.error
  const blobPending = cacheRemoteImage.isPending || uploadImage.isPending
  const attachPending = createImage.isPending || selectImage.isPending
  const target = { ownerType, ownerId, imageType, priority }

  async function handlePick(candidate: ArtworkCandidate) {
    setPendingUrl(candidate.url)
    try {
      const image = await attachFromUrl({ url: candidate.url, source: candidate.source, ...target })
      onAttached(image)
      onClose()
    } catch {
      // Error surfaced via cacheRemoteImage/createImage state below —
      // nothing further to do here.
    } finally {
      setPendingUrl(null)
    }
  }

  async function handleFileChange(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    const data = new Uint8Array(await readAsArrayBuffer(file))
    try {
      const image = await attachFromFile({ data, ...target })
      onAttached(image)
      onClose()
    } catch {
      // Error surfaced via uploadImage/createImage state below.
    }
  }

  return (
    <Modal title={title} onClose={onClose}>
      <div className="flex flex-col gap-4">
        {candidates.length > 0 && (
          <div className="flex flex-col gap-2">
            <span className="text-label text-text-secondary">Choose an image</span>
            <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
              {candidates.map(candidate => (
                <button
                  key={candidate.url}
                  type="button"
                  onClick={() => void handlePick(candidate)}
                  disabled={blobPending || attachPending}
                  aria-label={candidate.label ?? candidate.source}
                  className="group relative aspect-square overflow-hidden rounded-lg border border-border disabled:opacity-50"
                >
                  <img src={candidate.url} alt={candidate.label ?? candidate.source} className="h-full w-full object-cover" />
                  {pendingUrl === candidate.url && (blobPending || attachPending) && (
                    <div className="absolute inset-0 flex items-center justify-center bg-bg/70">
                      <Loader2 size={20} className="text-accent-system animate-spin" />
                    </div>
                  )}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="flex flex-col gap-2">
          <span className="text-label text-text-secondary">Or upload a file</span>
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={blobPending || attachPending}
            className="flex h-9 w-fit items-center gap-2 rounded-lg bg-surface border border-border px-4 text-body font-medium text-text hover:bg-surface-raised disabled:opacity-50"
          >
            {uploadImage.isPending ? <Loader2 size={16} className="animate-spin" /> : <Upload size={16} />}
            {uploadImage.isPending ? 'Uploading…' : 'Choose file…'}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            className="hidden"
            onChange={event => void handleFileChange(event)}
          />
        </div>

        {blobError && (
          <p className="text-status-failure text-body" role="alert">
            Couldn't fetch that image ({blobError.message}).
          </p>
        )}
        {createImage.isError && (
          <p className="text-status-failure text-body" role="alert">
            Image was stored but couldn't be attached ({createImage.error?.message}).
          </p>
        )}
        {createImage.isSuccess && selectImage.isError && (
          <p className="text-status-failure text-body" role="alert">
            Image was attached but couldn't be set as current ({selectImage.error?.message}).
          </p>
        )}
      </div>
    </Modal>
  )
}
