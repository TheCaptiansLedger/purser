import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { AlbumCard } from './AlbumCard'

describe('AlbumCard', () => {
  it('renders an imported album with artwork, a year subtitle, and its status badge', () => {
    render(<AlbumCard album={{ id: 'g1', title: 'Rumours', year: 1977, status: 'imported', imageId: 'img-1' }} />)

    expect(screen.getByText('Rumours')).toBeInTheDocument()
    expect(screen.getByText('1977')).toBeInTheDocument()
    expect(screen.getByAltText('Rumours')).toHaveAttribute('src', '/media/images/img-1')
    expect(screen.getByText('Imported')).toBeInTheDocument()
  })

  it('omits the year subtitle and artwork when neither is known', () => {
    render(<AlbumCard album={{ id: 'g2', title: 'Tusk', status: 'stub' }} />)

    expect(screen.getByText('Tusk')).toBeInTheDocument()
    expect(screen.queryByAltText('Tusk')).not.toBeInTheDocument()
    expect(screen.getByText('Stub')).toBeInTheDocument()
  })

  it('renders a defined "No edition selected" state for a Group with zero MusicRelease rows', () => {
    render(<AlbumCard album={{ id: 'g3', title: 'Unreleased Sessions' }} />)

    expect(screen.getByText('Unreleased Sessions')).toBeInTheDocument()
    expect(screen.getByText('No edition selected')).toBeInTheDocument()
  })
})
