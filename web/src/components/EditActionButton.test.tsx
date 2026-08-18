import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { EditActionButton } from './EditActionButton'

// ADR 0004: a shared component needs its test to exercise more than one
// prop configuration — here, the two states every future caller (#681's
// Person/Album editors) will actually be in: no editor built yet
// (onEdit omitted, primary segment disabled) and a single-provider menu
// (Artist Detail's own "Refresh from MusicBrainz", #671) vs. a
// multi-provider menu, proving the chevron segment is genuinely
// provider-count-agnostic rather than shaped around exactly one item.
describe('EditActionButton', () => {
  it('disables the primary Edit segment when onEdit is omitted, without disabling the menu', () => {
    const onSelect = vi.fn()
    render(<EditActionButton editDisabledReason="Manual editing isn't available yet" items={[{ label: 'Refresh from MusicBrainz', onSelect }]} />)

    const editButton = screen.getByRole('button', { name: 'Edit' })
    expect(editButton).toBeDisabled()
    expect(editButton).toHaveAttribute('title', "Manual editing isn't available yet")

    fireEvent.click(screen.getByRole('button', { name: 'More edit actions' }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'Refresh from MusicBrainz' }))
    expect(onSelect).toHaveBeenCalled()
  })

  it('enables the primary Edit segment and lists every provider action when given several', () => {
    const onEdit = vi.fn()
    const refreshA = vi.fn()
    const refreshB = vi.fn()
    render(
      <EditActionButton
        onEdit={onEdit}
        items={[
          { label: 'Refresh from MusicBrainz', onSelect: refreshA },
          { label: 'Refresh from TheAudioDB', onSelect: refreshB },
        ]}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Edit' }))
    expect(onEdit).toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'More edit actions' }))
    expect(screen.getByRole('menuitem', { name: 'Refresh from MusicBrainz' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('menuitem', { name: 'Refresh from TheAudioDB' }))
    expect(refreshB).toHaveBeenCalled()
  })
})
