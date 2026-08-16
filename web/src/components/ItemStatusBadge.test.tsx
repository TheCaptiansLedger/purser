import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { ItemStatusBadge } from './ItemStatusBadge'

describe('ItemStatusBadge', () => {
  it('renders a wanted item as Wanted, colored pending', () => {
    render(<ItemStatusBadge status="wanted" />)
    expect(screen.getByText('Wanted')).toHaveClass('text-status-pending')
  })

  it('renders a grabbed item as Grabbed, colored queued', () => {
    render(<ItemStatusBadge status="grabbed" />)
    expect(screen.getByText('Grabbed')).toHaveClass('text-status-queued')
  })

  it('renders a downloading item as Downloading, colored active', () => {
    render(<ItemStatusBadge status="downloading" />)
    expect(screen.getByText('Downloading')).toHaveClass('text-status-active')
  })

  it('renders an imported item as Imported, colored success', () => {
    render(<ItemStatusBadge status="imported" />)
    expect(screen.getByText('Imported')).toHaveClass('text-status-success')
  })

  it('renders a missing item as Missing, colored failure', () => {
    render(<ItemStatusBadge status="missing" />)
    expect(screen.getByText('Missing')).toHaveClass('text-status-failure')
  })

  it('renders a skipped item as Skipped, colored neutral', () => {
    render(<ItemStatusBadge status="skipped" />)
    expect(screen.getByText('Skipped')).toHaveClass('text-status-neutral')
  })
})
