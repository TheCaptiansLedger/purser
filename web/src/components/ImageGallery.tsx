import { useMutation, useQuery } from '@connectrpc/connect-query'
import { Check, ImageOff, Loader2, Trash2 } from 'lucide-react'
import { deleteImage, getSelectedImage, listImages, selectImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import { EmptyState } from './EmptyState'
import { Modal } from './Modal'

const GALLERY_PAGE_SIZE = 50

export interface ImageGalleryProps {
  title: string
  ownerType: string
  ownerId: string
  imageType: string
  onClose: () => void
  // Called after a successful Select or Delete — the caller's own "what's
  // the current image" query (e.g. useSelectedImage) is a separate call
  // this component doesn't know about, so it can't invalidate that cache
  // entry itself; this is how the caller finds out something changed.
  onChange?: () => void
}

// ImageGallery is the other half of the attach flow (#656/ChooseArtworkDialog
// only ever adds a new image): every Image ever attached to one
// owner+slot, which one is current, and two real actions per thumbnail —
// "Use this one" (SelectImage) and "Delete" (DeleteImage, which frees the
// underlying bytes too, see internal/service/image.go). Generic over
// ownerType/ownerId/imageType, same reuse contract every image-attach
// component in this codebase follows.
export function ImageGallery({ title, ownerType, ownerId, imageType, onClose, onChange }: ImageGalleryProps) {
  const listQuery = useQuery(listImages, { ownerType, ownerId, imageType, pageSize: GALLERY_PAGE_SIZE, pageToken: '' })
  const selectedQuery = useQuery(getSelectedImage, { ownerType, ownerId, imageType })
  const selectMutation = useMutation(selectImage)
  const deleteMutation = useMutation(deleteImage)

  const selectedId = selectedQuery.data?.image?.id
  const images = listQuery.data?.images ?? []

  function handleSelect(imageId: string) {
    selectMutation.mutate(
      { ownerType, ownerId, imageType, imageId },
      {
        onSuccess: () => {
          void selectedQuery.refetch()
          onChange?.()
        },
      },
    )
  }

  function handleDelete(imageId: string) {
    // Destructive and infrequent enough that a native confirm is enough
    // friction — same call DatabaseTab's restore action already made,
    // no dedicated confirm dialog exists in this codebase to justify
    // building one for this alone.
    if (!window.confirm('Delete this image? This cannot be undone.')) return
    deleteMutation.mutate(
      { id: imageId },
      {
        onSuccess: () => {
          void listQuery.refetch()
          void selectedQuery.refetch()
          onChange?.()
        },
      },
    )
  }

  return (
    <Modal title={title} onClose={onClose}>
      <div className="flex flex-col gap-4">
        {listQuery.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't load images ({listQuery.error.message}).
          </p>
        )}

        {!listQuery.isPending && !listQuery.isError && images.length === 0 && (
          <EmptyState icon={ImageOff} title="No images yet" description="Nothing has been attached to this slot." />
        )}

        {images.length > 0 && (
          <div className="grid grid-cols-3 gap-3 sm:grid-cols-4">
            {images.map(image => {
              const isSelected = image.id === selectedId
              const isBusy =
                (selectMutation.isPending && selectMutation.variables?.imageId === image.id) ||
                (deleteMutation.isPending && deleteMutation.variables?.id === image.id)
              return (
                <div
                  key={image.id}
                  className={[
                    'group relative aspect-square overflow-hidden rounded-lg border',
                    isSelected ? 'border-accent-system' : 'border-border',
                  ].join(' ')}
                >
                  <img src={`/media/images/${image.id}`} alt="" className="h-full w-full object-cover" />

                  {isSelected && (
                    <div className="absolute left-1.5 top-1.5 flex items-center gap-1 rounded-sm bg-accent-system px-1.5 py-0.5 text-label font-medium text-bg">
                      <Check size={12} />
                      Current
                    </div>
                  )}

                  <div className="absolute inset-x-0 bottom-0 flex items-center justify-between gap-1 bg-bg/80 p-1.5 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                    {!isSelected && (
                      <button
                        type="button"
                        onClick={() => handleSelect(image.id)}
                        disabled={isBusy}
                        className="h-7 rounded-md px-2 text-label font-medium text-text hover:bg-surface-raised disabled:opacity-50"
                      >
                        Use this one
                      </button>
                    )}
                    <button
                      type="button"
                      onClick={() => handleDelete(image.id)}
                      disabled={isBusy}
                      aria-label="Delete image"
                      className="ml-auto flex h-7 w-7 items-center justify-center rounded-md text-status-failure hover:bg-surface-raised disabled:opacity-50"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>

                  {isBusy && (
                    <div className="absolute inset-0 flex items-center justify-center bg-bg/70">
                      <Loader2 size={20} className="animate-spin text-accent-system" />
                    </div>
                  )}
                </div>
              )
            })}
          </div>
        )}

        {selectMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't switch the current image ({selectMutation.error?.message}).
          </p>
        )}
        {deleteMutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't delete that image ({deleteMutation.error?.message}).
          </p>
        )}
      </div>
    </Modal>
  )
}
