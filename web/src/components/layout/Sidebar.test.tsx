import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { Sidebar } from './Sidebar'

function renderSidebar(props: Partial<React.ComponentProps<typeof Sidebar>> = {}) {
  const onCollapsedChange = vi.fn()
  render(
    <MemoryRouter>
      <Sidebar collapsed={false} onCollapsedChange={onCollapsedChange} {...props} />
    </MemoryRouter>,
  )
  return { onCollapsedChange }
}

describe('Sidebar', () => {
  it('shows the nav label when expanded', () => {
    renderSidebar({ collapsed: false })
    expect(screen.getByRole('link', { name: 'Welcome' })).toBeInTheDocument()
  })

  it('hides the nav label but keeps the icon reachable when collapsed', () => {
    renderSidebar({ collapsed: true })
    expect(screen.queryByText('Welcome')).not.toBeInTheDocument()
    expect(screen.getByRole('link')).toBeInTheDocument()
  })

  it('calls onCollapsedChange when the toggle is clicked', () => {
    const { onCollapsedChange } = renderSidebar({ collapsed: false })
    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))
    expect(onCollapsedChange).toHaveBeenCalledWith(true)
  })

  it('translates off-canvas when not mobileOpen', () => {
    renderSidebar({ mobileOpen: false })
    expect(screen.getByRole('link', { name: 'Welcome' }).closest('aside')).toHaveClass('-translate-x-full')
  })
})
