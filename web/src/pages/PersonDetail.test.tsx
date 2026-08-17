import type { ConnectRouter } from '@connectrpc/connect'
import { createRouterTransport } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { timestampFromDate } from '@bufbuild/protobuf/wkt'
import { describe, expect, it } from 'vitest'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { EntryPersonService } from '../gen/purser/domain/v1/entry_person_pb'
import { ImageBlobService } from '../gen/purser/domain/v1/image_blob_pb'
import { ImageService } from '../gen/purser/domain/v1/image_pb'
import { ItemPersonService } from '../gen/purser/domain/v1/item_person_pb'
import { Gender, PersonService } from '../gen/purser/domain/v1/person_pb'
import { PersonDetail } from './PersonDetail'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern as
// ChooseArtworkDialog.test.tsx/ImageGallery.test.tsx, which the
// photo-attach and photo-management assertions below exercise unmodified
// (composition, not a re-test of their own error states — those already
// have their own tests).
function renderPersonDetail(mockTransport: ReturnType<typeof createRouterTransport>, id = 'person-1') {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <MemoryRouter initialEntries={[`/people/${id}`]}>
          <Routes>
            <Route path="/people/:id" element={<PersonDetail />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    </TransportProvider>,
  )
}

const basePerson = {
  id: 'person-1',
  name: 'Stevie Nicks',
  sortName: 'Nicks, Stevie',
  aliases: [],
  gender: Gender.FEMALE,
  pronouns: 'she/her',
  birthDate: timestampFromDate(new Date('1948-05-26T12:00:00Z')),
  deathDate: undefined,
  nationality: 'American',
  overview: '',
  monitored: false,
  monitorMode: MonitorMode.NONE,
  addedAt: undefined,
  updatedAt: undefined,
}

function noSelectedImage() {
  return () => {
    throw new Error('not found')
  }
}

// registerNoAppearances — #662's PersonAppearances renders unconditionally
// below the facts panel, so every test that renders past GetPerson needs
// EntryPersonService/ItemPersonService wired to something; empty lists
// keep these tests focused on what they actually assert.
function registerNoAppearances(router: ConnectRouter) {
  router.service(EntryPersonService, { listEntryPeople: () => ({ entryPeople: [], nextPageToken: '' }) })
  router.service(ItemPersonService, { listItemPeople: () => ({ itemPeople: [], nextPageToken: '' }) })
}

describe('PersonDetail — load', () => {
  it('renders the facts panel from GetPerson', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, { getPerson: () => ({ person: basePerson }) })
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      registerNoAppearances(router)
    })

    renderPersonDetail(mockTransport)

    await waitFor(() => expect(screen.getByText('Stevie Nicks')).toBeInTheDocument())
    expect(screen.getByText('she/her')).toBeInTheDocument()
    expect(screen.getByText('American')).toBeInTheDocument()
    expect(screen.getByText(/Born May 26, 1948/)).toBeInTheDocument()
  })

  it('shows an actionable error when GetPerson fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPerson: () => {
          throw new Error('not found')
        },
      })
    })

    renderPersonDetail(mockTransport)

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent("Couldn't load this person"))
  })
})

describe('PersonDetail — monitor toggle round-trip', () => {
  it('flips optimistically on click, then reconciles with the UpdatePerson response', async () => {
    let resolveUpdate: (() => void) | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPerson: () => ({ person: basePerson }),
        updatePerson: req => {
          expect(req.updateMask?.paths).toEqual(['monitored', 'monitor_mode'])
          return new Promise(resolve => {
            resolveUpdate = () =>
              resolve({ person: { ...basePerson, monitored: true, monitorMode: MonitorMode.ALL } })
          })
        },
      })
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      registerNoAppearances(router)
    })

    renderPersonDetail(mockTransport)
    await waitFor(() => expect(screen.getByText('Stevie Nicks')).toBeInTheDocument())

    const toggle = screen.getByRole('switch', { name: 'Monitored' })
    expect(toggle).toHaveAttribute('aria-checked', 'false')

    fireEvent.click(toggle)
    // Optimistic: flips before the mutation's promise has resolved.
    expect(toggle).toHaveAttribute('aria-checked', 'true')

    resolveUpdate?.()
    // Server confirmation: still true once the response lands.
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'true'))
  })

  it('rolls back to the previous state when UpdatePerson fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, {
        getPerson: () => ({ person: basePerson }),
        updatePerson: () => {
          throw new Error('unavailable')
        },
      })
      router.service(ImageService, { getSelectedImage: noSelectedImage() })
      registerNoAppearances(router)
    })

    renderPersonDetail(mockTransport)
    await waitFor(() => expect(screen.getByText('Stevie Nicks')).toBeInTheDocument())

    const toggle = screen.getByRole('switch', { name: 'Monitored' })
    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-checked', 'true')

    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'false'))
  })
})

describe('PersonDetail — photo attach and lightbox', () => {
  it('attaches an uploaded photo via ChooseArtworkDialog (which also selects it), then opens it in the lightbox on click', async () => {
    // selectedImageId models server-side state: unset until SelectImage is
    // called, same as a real ImageService would track it.
    let selectedImageId: string | undefined
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, { getPerson: () => ({ person: basePerson }) })
      router.service(ImageService, {
        getSelectedImage: () => {
          if (!selectedImageId) throw new Error('not found')
          return { image: { id: selectedImageId } }
        },
        createImage: req => {
          expect(req.image?.ownerType).toBe('person')
          expect(req.image?.ownerId).toBe('person-1')
          expect(req.image?.imageType).toBe('photo')
          return { image: { ...req.image!, id: 'image-1' } }
        },
        selectImage: req => {
          selectedImageId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      router.service(ImageBlobService, {
        uploadImage: () => ({
          blob: { key: 'person/ab/img-1.jpg', width: 300, height: 300, contentType: 'image/jpeg', sizeBytes: 1000n },
        }),
      })
      registerNoAppearances(router)
    })

    renderPersonDetail(mockTransport)
    await waitFor(() => expect(screen.getByText('Stevie Nicks')).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: 'Add photo' }))
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement
    const file = new File([new Uint8Array([1, 2, 3])], 'photo.jpg', { type: 'image/jpeg' })
    fireEvent.change(fileInput, { target: { files: [file] } })

    const photoButton = await screen.findByRole('button', { name: "View Stevie Nicks's photo" })
    expect(photoButton.querySelector('img')).toHaveAttribute('src', '/media/images/image-1')

    fireEvent.click(photoButton)
    expect(screen.getByRole('dialog', { name: 'Stevie Nicks' })).toBeInTheDocument()
  })
})

describe('PersonDetail — manage photos', () => {
  it('opens the gallery, switches the current photo, and reflects the change on the page', async () => {
    let selectedImageId = 'image-old'
    const mockTransport = createRouterTransport(router => {
      router.service(PersonService, { getPerson: () => ({ person: basePerson }) })
      router.service(ImageService, {
        getSelectedImage: () => ({ image: { id: selectedImageId } }),
        listImages: () => ({ images: [{ id: 'image-old' }, { id: 'image-new' }], nextPageToken: '' }),
        selectImage: req => {
          selectedImageId = req.imageId
          return { image: { id: req.imageId } }
        },
      })
      registerNoAppearances(router)
    })

    renderPersonDetail(mockTransport)
    await waitFor(() => expect(screen.getByText('Stevie Nicks')).toBeInTheDocument())
    expect(screen.getByRole('button', { name: "View Stevie Nicks's photo" }).querySelector('img')).toHaveAttribute(
      'src',
      '/media/images/image-old',
    )

    fireEvent.click(screen.getByRole('button', { name: 'Manage photos' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Use this one' })).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Use this one' }))

    await waitFor(() =>
      expect(screen.getByRole('button', { name: "View Stevie Nicks's photo" }).querySelector('img')).toHaveAttribute(
        'src',
        '/media/images/image-new',
      ),
    )
  })
})
