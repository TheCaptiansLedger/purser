import type { ReactNode } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { SelectableTile } from './SelectableTile'

// ADR 0004: a shared component's test exercises it with more than one
// content type — an ArtistCard-shaped tile and an AlbumCard-shaped one —
// to prove SelectableTile itself carries no content-type branching.
function renderTile(
  props: Partial<React.ComponentProps<typeof SelectableTile>> = {},
  children: ReactNode = <span>Fleetwood Mac</span>,
) {
  const onToggle = vi.fn()
  render(
    <MemoryRouter initialEntries={['/music']}>
      <Routes>
        <Route
          path="/music"
          element={
            <SelectableTile to="/music/artists/a1" selectMode={false} selected={false} onToggle={onToggle} {...props}>
              {children}
            </SelectableTile>
          }
        />
        <Route path="/music/artists/a1" element={<div>Artist detail page</div>} />
      </Routes>
    </MemoryRouter>,
  )
  return { onToggle }
}

describe('SelectableTile', () => {
  it('renders a plain navigating Link when not in select mode', () => {
    renderTile({ selectMode: false })

    fireEvent.click(screen.getByText('Fleetwood Mac'))

    expect(screen.getByText('Artist detail page')).toBeInTheDocument()
  })

  it('renders a toggle button instead of navigating when in select mode', () => {
    const { onToggle } = renderTile({ selectMode: true, selected: false })

    fireEvent.click(screen.getByText('Fleetwood Mac'))

    expect(onToggle).toHaveBeenCalledOnce()
    expect(screen.queryByText('Artist detail page')).not.toBeInTheDocument()
  })

  it('reflects the selected state via aria-pressed for an album-shaped child too', () => {
    renderTile({ selectMode: true, selected: true }, <span>Rumours</span>)

    expect(screen.getByRole('button', { pressed: true })).toBeInTheDocument()
  })

  it('onNavigate mode: navigates on a plain click but not on a click landing on a nested button (PersonCard-shaped)', () => {
    const onNavigate = vi.fn()
    render(
      <SelectableTile
        selectMode={false}
        selected={false}
        onToggle={vi.fn()}
        onNavigate={onNavigate}
        ariaLabel="View Stevie Nicks"
      >
        <span>Stevie Nicks</span>
        <button type="button">View photo</button>
      </SelectableTile>,
    )

    fireEvent.click(screen.getByRole('button', { name: 'View photo' }))
    expect(onNavigate).not.toHaveBeenCalled()

    fireEvent.click(screen.getByText('Stevie Nicks'))
    expect(onNavigate).toHaveBeenCalledOnce()
  })

  it('onNavigate mode: select mode still swaps in the toggle button, not the click-target div', () => {
    const onToggle = vi.fn()
    render(
      <SelectableTile selectMode selected={false} onToggle={onToggle} onNavigate={vi.fn()} ariaLabel="View Stevie Nicks">
        <span>Stevie Nicks</span>
      </SelectableTile>,
    )

    fireEvent.click(screen.getByText('Stevie Nicks'))
    expect(onToggle).toHaveBeenCalledOnce()
  })
})
