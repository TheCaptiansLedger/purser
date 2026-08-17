import { useQuery } from '@connectrpc/connect-query'
import { Camera, Images, User } from 'lucide-react'
import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { timestampDate } from '@bufbuild/protobuf/wkt'
import type { Timestamp } from '@bufbuild/protobuf/wkt'
import { ChooseArtworkDialog } from '../components/ChooseArtworkDialog'
import { ImageGallery } from '../components/ImageGallery'
import { ImageLightbox } from '../components/ImageLightbox'
import { PersonAppearances } from '../components/PersonAppearances'
import { Toggle } from '../components/Toggle'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { getSelectedImage } from '../gen/purser/domain/v1/image-ImageService_connectquery'
import { usePerson, useUpdatePersonMutation } from '../hooks/usePerson'

function formatDate(timestamp: Timestamp | undefined): string | undefined {
  if (!timestamp) return undefined
  return timestampDate(timestamp).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' })
}

// PersonDetail — the Person Detail page (#661). PersonService.GetPerson,
// a facts panel (birth/death, nationality, pronouns), a Monitor toggle
// (UpdatePerson field-mask [monitored, monitor_mode]), and the photo
// flow: #656's ChooseArtworkDialog to attach a new photo (which also
// selects it — see useAttachImage), ImageGallery to see every previously
// attached photo and switch which one is current or delete one, and
// #655's ImageLightbox to view the current one full-screen on click.
// Upload-only in ChooseArtworkDialog — unlike Artist/Album art there's no
// provider lookup RPC for a Person photo, so no candidates are passed.
// #662's PersonAppearances renders the "Appears as" cross-module list
// below the facts panel.
export function PersonDetail() {
  const { id = '' } = useParams<{ id: string }>()
  const personQuery = usePerson(id)
  const updatePersonMutation = useUpdatePersonMutation()
  const selectedImageQuery = useQuery(
    getSelectedImage,
    { ownerType: 'person', ownerId: id, imageType: 'photo' },
    { enabled: !!id, retry: false },
  )

  // Optimistic local mirror of the one server-owned field this page can
  // write directly — set immediately on toggle, then reconciled from the
  // UpdatePerson response (server confirmation) or rolled back on error.
  const [monitored, setMonitored] = useState<boolean | undefined>(undefined)
  const [lightboxOpen, setLightboxOpen] = useState(false)
  const [artworkDialogOpen, setArtworkDialogOpen] = useState(false)
  const [galleryOpen, setGalleryOpen] = useState(false)

  // Doherty threshold — see docs/design/ux-principles.md#feedback--system-status.
  // A local Connect round trip resolves well under 400ms; a loading
  // indicator here would read as slower, not more informative.
  if (personQuery.isPending) {
    return null
  }

  if (personQuery.isError) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Couldn't load this person ({personQuery.error.message}).
      </p>
    )
  }

  const person = personQuery.data.person
  if (!person) {
    return (
      <p className="mx-6 mt-10 text-body text-status-failure" role="alert">
        Person not found.
      </p>
    )
  }

  const isMonitored = monitored ?? person.monitored
  const imageId = selectedImageQuery.data?.image?.id
  const imageSrc = imageId ? `/media/images/${imageId}` : undefined

  function handleMonitorToggle(next: boolean) {
    const previous = isMonitored
    setMonitored(next)
    updatePersonMutation.mutate(
      {
        // Only id (for lookup) plus the two masked fields — every other
        // field on Person is ignored server-side for paths outside the
        // mask (see applyPersonFieldMask), so there's nothing to gain by
        // spreading the full fetched Person into this request.
        person: { id, monitored: next, monitorMode: next ? MonitorMode.ALL : MonitorMode.NONE },
        updateMask: { paths: ['monitored', 'monitor_mode'] },
      },
      {
        onSuccess: response => setMonitored(response.person?.monitored ?? next),
        onError: () => setMonitored(previous),
      },
    )
  }

  const facts = [
    person.pronouns,
    person.nationality,
    person.birthDate && `Born ${formatDate(person.birthDate)}`,
    person.deathDate && `Died ${formatDate(person.deathDate)}`,
  ].filter((fact): fact is string => !!fact)

  return (
    <div className="px-6 py-10 md:px-8">
      <div className="flex flex-col gap-6 sm:flex-row sm:items-start">
        <div className="flex flex-col items-center gap-2 sm:w-48 shrink-0">
          {imageSrc ? (
            <button
              type="button"
              onClick={() => setLightboxOpen(true)}
              aria-label={`View ${person.name}'s photo`}
              className="aspect-square w-full overflow-hidden rounded-full border border-border"
            >
              <img src={imageSrc} alt={person.name} className="h-full w-full object-cover" />
            </button>
          ) : (
            <div
              aria-hidden="true"
              className="flex aspect-square w-full items-center justify-center rounded-full border border-border bg-surface-raised text-text-secondary"
            >
              <User size={40} />
            </div>
          )}

          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => setArtworkDialogOpen(true)}
              className="flex h-8 items-center gap-2 rounded-lg px-3 text-label font-medium text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Camera size={14} />
              {imageSrc ? 'Change photo' : 'Add photo'}
            </button>

            <button
              type="button"
              onClick={() => setGalleryOpen(true)}
              aria-label="Manage photos"
              title="Manage photos"
              className="flex h-8 w-8 items-center justify-center rounded-lg text-text-secondary hover:bg-surface-raised hover:text-text"
            >
              <Images size={14} />
            </button>
          </div>
        </div>

        <div className="flex flex-1 flex-col gap-4">
          <h1 className="text-headline text-text">{person.name}</h1>

          <Toggle
            label="Monitored"
            checked={isMonitored}
            onChange={handleMonitorToggle}
            disabled={updatePersonMutation.isPending}
          />

          {facts.length > 0 && (
            <div className="flex max-w-sm flex-col gap-2 rounded-xl border border-border bg-surface p-4">
              {facts.map(fact => (
                <span key={fact} className="text-body text-text-secondary">
                  {fact}
                </span>
              ))}
            </div>
          )}

          <PersonAppearances personId={id} />
        </div>
      </div>

      {lightboxOpen && imageSrc && (
        <ImageLightbox src={imageSrc} alt={person.name} onClose={() => setLightboxOpen(false)} />
      )}

      {artworkDialogOpen && (
        <ChooseArtworkDialog
          title={`${imageSrc ? 'Change' : 'Add'} photo`}
          ownerType="person"
          ownerId={id}
          imageType="photo"
          onClose={() => setArtworkDialogOpen(false)}
          onAttached={() => void selectedImageQuery.refetch()}
        />
      )}

      {galleryOpen && (
        <ImageGallery
          title="Manage photos"
          ownerType="person"
          ownerId={id}
          imageType="photo"
          onClose={() => setGalleryOpen(false)}
          onChange={() => void selectedImageQuery.refetch()}
        />
      )}
    </div>
  )
}
