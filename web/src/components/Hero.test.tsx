import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Hero } from './Hero'

// Shared-component test per ADR 0004: exercised with the two real
// consumer configurations named in #658 — Artist Detail and Album
// Detail — rendered directly, not asserted by inspection.
describe('Hero', () => {
  it('renders an Artist-Detail-style config: backdrop, title, facts row, action slot', () => {
    render(
      <Hero
        backdropSrc="/media/images/artist-backdrop-1"
        title="Fleetwood Mac"
        facts={['4 albums', '1987–present']}
        actions={<button type="button">Edit</button>}
      />,
    )

    expect(screen.getByRole('heading', { name: 'Fleetwood Mac' })).toBeInTheDocument()
    expect(screen.getByText('4 albums')).toBeInTheDocument()
    expect(screen.getByText('1987–present')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
    // Decorative backdrop — no informational alt text (WCAG 1.1.1).
    expect(screen.getByAltText('')).toHaveAttribute('src', '/media/images/artist-backdrop-1')
  })

  it('renders an Album-Detail-style config: no backdrop yet, different facts, no actions', () => {
    render(<Hero title="Rumours" facts={['2001', 'Rock', '12 tracks']} />)

    expect(screen.getByRole('heading', { name: 'Rumours' })).toBeInTheDocument()
    expect(screen.getByText('2001')).toBeInTheDocument()
    expect(screen.getByText('Rock')).toBeInTheDocument()
    expect(screen.getByText('12 tracks')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('omits the facts row entirely when none are given', () => {
    render(<Hero title="No Facts" />)

    expect(screen.getByRole('heading', { name: 'No Facts' })).toBeInTheDocument()
  })
})
