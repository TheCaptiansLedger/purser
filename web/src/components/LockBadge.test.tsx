import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { LockBadge } from './LockBadge'

describe('LockBadge', () => {
  it('tells the operator-locked story', () => {
    render(<LockBadge reason="operator" />)
    expect(screen.getByText('Set via env/yaml')).toBeInTheDocument()
  })

  it('tells the bootstrap-locked story', () => {
    render(<LockBadge reason="bootstrap" />)
    expect(screen.getByText('Requires restart')).toBeInTheDocument()
  })
})
