import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { InfoPopover } from './InfoPopover'

describe('InfoPopover', () => {
  it('stays closed until the trigger is clicked, for an AfterDark field set', () => {
    render(
      <InfoPopover
        title="AfterDark Rename Template Fields"
        triggerLabel="Show available template fields"
        fields={[{ name: 'SceneTitle', example: 'Scene Title' }]}
      />,
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Show available template fields' }))
    expect(screen.getByRole('dialog', { name: 'AfterDark Rename Template Fields' })).toBeInTheDocument()
    expect(screen.getByText('{{.SceneTitle}}')).toBeInTheDocument()
    expect(screen.getByText('Scene Title')).toBeInTheDocument()
  })

  it('renders a different field set for Music, proving it is generic over content', () => {
    render(
      <InfoPopover
        title="Music Rename Template Fields"
        triggerLabel="Show available template fields"
        fields={[
          { name: 'ArtistName', example: 'Artist Name' },
          { name: 'TrackNumber', example: '7' },
        ]}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Show available template fields' }))
    expect(screen.getByText('{{.ArtistName}}')).toBeInTheDocument()
    expect(screen.getByText('{{.TrackNumber}}')).toBeInTheDocument()
    expect(screen.queryByText('{{.SceneTitle}}')).not.toBeInTheDocument()
  })

  it('closes via the Modal close button', () => {
    render(
      <InfoPopover title="AfterDark Rename Template Fields" triggerLabel="Show available template fields" fields={[]} />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Show available template fields' }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
