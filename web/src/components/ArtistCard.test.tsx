import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ArtistCard } from './ArtistCard'

describe('ArtistCard', () => {
  it('renders a monitored artist with artwork and a genre subtitle', () => {
    render(
      <ArtistCard artist={{ id: 'a1', name: 'Fleetwood Mac', monitored: true, genre: 'Rock', imageId: 'img-1' }} />,
    )

    expect(screen.getByText('Fleetwood Mac')).toBeInTheDocument()
    expect(screen.getByText('Rock')).toBeInTheDocument()
    expect(screen.getByAltText('Fleetwood Mac')).toHaveAttribute('src', '/media/images/img-1')
    expect(screen.getByRole('img', { name: 'Monitored' })).toBeInTheDocument()
  })

  it('renders a not-monitored artist with no genre and no artwork as a placeholder', () => {
    render(<ArtistCard artist={{ id: 'a2', name: 'Steely Dan', monitored: false }} />)

    expect(screen.getByText('Steely Dan')).toBeInTheDocument()
    expect(screen.queryByAltText('Steely Dan')).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Not monitored' })).toBeInTheDocument()
  })

  it('renders no genre chip at all when Metadata has no genre, not an empty placeholder', () => {
    const { container } = render(<ArtistCard artist={{ id: 'a3', name: 'No Genre', monitored: true }} />)

    expect(screen.getByText('No Genre')).toBeInTheDocument()
    expect(container.querySelector('.text-text-secondary.truncate')).not.toBeInTheDocument()
  })

  it('renders the ownership ring (#670) alongside the monitored dot when given one', () => {
    render(
      <ArtistCard
        artist={{ id: 'a4', name: 'Steely Dan', monitored: true }}
        ownership={{ owned: 4, total: 7 }}
      />,
    )

    expect(screen.getByRole('img', { name: '4 of 7 albums owned' })).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Monitored' })).toBeInTheDocument()
  })

  it('renders no ownership ring at all when no ownership prop is given', () => {
    render(<ArtistCard artist={{ id: 'a5', name: 'No Ownership Data', monitored: true }} />)

    expect(screen.queryByText(/\d+\/\d+/)).not.toBeInTheDocument()
  })
})
