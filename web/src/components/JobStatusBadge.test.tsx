import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { JobStatusBadge } from './JobStatusBadge'

describe('JobStatusBadge', () => {
  it('renders a running job as Running, colored active', () => {
    render(<JobStatusBadge status="running" />)
    expect(screen.getByText('Running')).toHaveClass('text-status-active')
  })

  it('renders a failed job as Failed, colored failure', () => {
    render(<JobStatusBadge status="failed" />)
    expect(screen.getByText('Failed')).toHaveClass('text-status-failure')
  })

  it('renders a partial job as Partial, colored warning', () => {
    render(<JobStatusBadge status="partial" />)
    expect(screen.getByText('Partial')).toHaveClass('text-status-warning')
  })
})
