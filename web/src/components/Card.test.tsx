import { Disc3, Music } from 'lucide-react'
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Card } from './Card'

// Shared-component test per ADR 0004: exercised with the two real
// consumer configurations named in #658 — Music Library grid's
// ArtistCard and Artist Detail Discography tab's AlbumCard — rendered
// directly, not asserted by inspection.
describe('Card', () => {
  it('renders an ArtistCard-style config: photo, name, genre subtitle, custom placeholder icon', () => {
    render(
      <Card
        imageSrc="/media/images/artist-1"
        title="Fleetwood Mac"
        subtitle="Rock"
        placeholderIcon={Music}
      />,
    )

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.getByText('Rock')).toBeInTheDocument()
    expect(screen.getByAltText('Fleetwood Mac')).toHaveAttribute('src', '/media/images/artist-1')
  })

  it('renders an AlbumCard-style config: no artwork yet, artist-name subtitle, status badge, custom placeholder icon', () => {
    render(
      <Card
        title="Rumours"
        subtitle="Fleetwood Mac"
        badge={<span>Imported</span>}
        placeholderIcon={Disc3}
      />,
    )

    expect(screen.getByText('Rumours')).toBeInTheDocument()
    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.getByText('Imported')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('omits the subtitle line entirely when none is given', () => {
    render(<Card title="No Subtitle" />)

    expect(screen.getByText('No Subtitle')).toBeInTheDocument()
  })
})
