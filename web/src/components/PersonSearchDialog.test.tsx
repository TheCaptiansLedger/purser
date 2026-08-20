import { createRouterTransport, ConnectError, Code } from '@connectrpc/connect'
import { TransportProvider } from '@connectrpc/connect-query'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ExternalIDService } from '../gen/purser/domain/v1/external_id_pb'
import { PersonService } from '../gen/purser/domain/v1/person_pb'
import { MusicBrainzService } from '../gen/purser/music/v1/musicbrainz_search_pb'
import { PersonSearchDialog } from './PersonSearchDialog'

// ADR 0004: the API layer is tested against a mocked transport, never a
// live backend — same createRouterTransport pattern AddArtistDialog.test.tsx
// uses. useGetOrCreatePersonFromMusicBrainz's own get-or-create
// composition (hit/win/lose) is covered by its own hook test; this file
// only proves the dialog wires search → Person-only filter → selection →
// the hook → onAdded correctly.
function renderDialog(mockTransport: ReturnType<typeof createRouterTransport>) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const onClose = vi.fn()
  const onAdded = vi.fn()
  render(
    <TransportProvider transport={mockTransport}>
      <QueryClientProvider client={queryClient}>
        <PersonSearchDialog onClose={onClose} onAdded={onAdded} />
      </QueryClientProvider>
    </TransportProvider>,
  )
  return { onClose, onAdded }
}

describe('PersonSearchDialog', () => {
  it('searches MusicBrainz, filters to type=="Person", and gets-or-creates the picked result', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: req => {
          expect(req.query).toBe('stevie nicks')
          return {
            artists: [
              { mbid: 'mbid-band', name: 'Fleetwood Mac', sortName: 'Fleetwood Mac', type: 'Group' },
              {
                mbid: 'mbid-1',
                name: 'Stevie Nicks',
                sortName: 'Nicks, Stevie',
                type: 'Person',
                country: 'US',
                lifeSpanBegin: '1948-05-26',
              },
            ],
          }
        },
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('not found', Code.NotFound)
        },
        createExternalID: req => ({ externalId: { entityId: req.externalId!.entityId } }),
      })
      router.service(PersonService, {
        createPerson: req => {
          expect(req.person?.nationality).toBe('US')
          return { person: { id: 'person-nicks', name: req.person!.name } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for a person'), {
      target: { value: 'stevie nicks' },
    })

    await waitFor(() => expect(screen.getByRole('button', { name: /Stevie Nicks/ })).toBeInTheDocument(), {
      timeout: 2000,
    })
    expect(screen.queryByText('Fleetwood Mac')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /Stevie Nicks/ }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('person-nicks'))
  })

  it('reuses an already-linked Person instead of creating a duplicate', async () => {
    let createCalled = false
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: () => ({
          artists: [{ mbid: 'mbid-1', name: 'Stevie Nicks', sortName: 'Nicks, Stevie', type: 'Person' }],
        }),
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => ({ externalId: { entityId: 'existing-person-1' } }),
      })
      router.service(PersonService, {
        createPerson: () => {
          createCalled = true
          return { person: { id: 'should-not-happen' } }
        },
      })
    })
    const { onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for a person'), { target: { value: 'stevie' } })
    await waitFor(() => expect(screen.getByRole('button', { name: /Stevie Nicks/ })).toBeInTheDocument(), {
      timeout: 2000,
    })
    fireEvent.click(screen.getByRole('button', { name: /Stevie Nicks/ }))

    await waitFor(() => expect(onAdded).toHaveBeenCalledWith('existing-person-1'))
    expect(createCalled).toBe(false)
  })

  it('shows "No matches" when results exist but none are type=="Person"', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: () => ({ artists: [{ mbid: 'mbid-band', name: 'Fleetwood Mac', sortName: 'Fleetwood Mac', type: 'Group' }] }),
      })
    })
    renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for a person'), { target: { value: 'fleetwood' } })

    await waitFor(() => expect(screen.getByText('No matches')).toBeInTheDocument(), { timeout: 2000 })
  })

  it('shows an inline error and stays open when the get-or-create composition fails', async () => {
    const mockTransport = createRouterTransport(router => {
      router.service(MusicBrainzService, {
        searchArtists: () => ({
          artists: [{ mbid: 'mbid-1', name: 'Stevie Nicks', sortName: 'Nicks, Stevie', type: 'Person' }],
        }),
      })
      router.service(ExternalIDService, {
        getExternalIDByValue: () => {
          throw new ConnectError('lookup failed', Code.Internal)
        },
      })
    })
    const { onClose, onAdded } = renderDialog(mockTransport)

    fireEvent.change(screen.getByLabelText('Search MusicBrainz for a person'), { target: { value: 'stevie' } })
    await waitFor(() => expect(screen.getByRole('button', { name: /Stevie Nicks/ })).toBeInTheDocument(), {
      timeout: 2000,
    })
    fireEvent.click(screen.getByRole('button', { name: /Stevie Nicks/ }))

    await waitFor(() => expect(screen.getByRole('alert')).toBeInTheDocument())
    expect(onAdded).not.toHaveBeenCalled()
    expect(onClose).not.toHaveBeenCalled()
  })
})
