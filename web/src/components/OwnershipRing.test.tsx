import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { OwnershipRing } from './OwnershipRing'

describe('OwnershipRing', () => {
  it('renders the fraction of albums owned', () => {
    render(<OwnershipRing owned={4} total={7} />)

    expect(screen.getByRole('img', { name: '4 of 7 albums owned' })).toBeInTheDocument()
    expect(screen.getByText('4/7')).toBeInTheDocument()
  })

  it('renders a full ring without overflowing when owned equals total', () => {
    render(<OwnershipRing owned={3} total={3} />)

    expect(screen.getByRole('img', { name: '3 of 3 albums owned' })).toBeInTheDocument()
    expect(screen.getByText('3/3')).toBeInTheDocument()
  })

  it('renders a skeleton while the fan-out is pending, not the fraction', () => {
    render(<OwnershipRing owned={0} total={0} isPending />)

    expect(screen.getByRole('img', { name: 'Loading album ownership' })).toBeInTheDocument()
    expect(screen.queryByText(/\//)).not.toBeInTheDocument()
  })

  it("renders a defined error state, not a blank one, when this card's fan-out failed", () => {
    render(<OwnershipRing owned={0} total={0} isError />)

    expect(screen.getByRole('img', { name: "Couldn't load album ownership" })).toBeInTheDocument()
  })

  it('renders nothing for an artist with zero albums', () => {
    const { container } = render(<OwnershipRing owned={0} total={0} />)

    expect(container).toBeEmptyDOMElement()
  })
})
