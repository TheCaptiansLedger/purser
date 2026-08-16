import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { MusicReleaseStatusBadge } from './MusicReleaseStatusBadge'

describe('MusicReleaseStatusBadge', () => {
  it('renders a stub release as Stub, colored pending', () => {
    render(<MusicReleaseStatusBadge status="stub" />)
    expect(screen.getByText('Stub')).toHaveClass('text-status-pending')
  })

  it('renders a partial release as Partial, colored warning', () => {
    render(<MusicReleaseStatusBadge status="partial" />)
    expect(screen.getByText('Partial')).toHaveClass('text-status-warning')
  })

  it('renders an imported release as Imported, colored success', () => {
    render(<MusicReleaseStatusBadge status="imported" />)
    expect(screen.getByText('Imported')).toHaveClass('text-status-success')
  })
})
