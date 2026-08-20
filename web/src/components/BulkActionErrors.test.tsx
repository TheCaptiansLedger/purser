import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { BulkActionErrors } from './BulkActionErrors'

describe('BulkActionErrors', () => {
  it('renders nothing when there are no errors', () => {
    const { container } = render(<BulkActionErrors errors={[]} />)

    expect(container).toBeEmptyDOMElement()
  })

  it('renders every failed row individually, by name and message', () => {
    render(
      <BulkActionErrors
        errors={[
          { id: 'a1', label: 'Radiohead', message: 'not found' },
          { id: 'a2', label: 'Portishead', message: 'network error' },
        ]}
      />,
    )

    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't update 2 items")
    expect(screen.getByText('Radiohead: not found')).toBeInTheDocument()
    expect(screen.getByText('Portishead: network error')).toBeInTheDocument()
  })

  it('singularizes the summary at exactly one failure', () => {
    render(<BulkActionErrors errors={[{ id: 'a1', label: 'Radiohead', message: 'not found' }]} />)

    expect(screen.getByRole('alert')).toHaveTextContent("Couldn't update 1 item")
  })
})
