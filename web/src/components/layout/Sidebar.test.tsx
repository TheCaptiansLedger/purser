import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'
import { Sidebar } from './Sidebar'

function renderSidebar(props: Partial<React.ComponentProps<typeof Sidebar>> = {}, initialEntries: string[] = ['/']) {
  const onCollapsedChange = vi.fn()
  render(
    <MemoryRouter initialEntries={initialEntries}>
      <Sidebar collapsed={false} onCollapsedChange={onCollapsedChange} {...props} />
    </MemoryRouter>,
  )
  return { onCollapsedChange }
}

describe('Sidebar', () => {
  it('shows the nav labels when expanded', () => {
    renderSidebar({ collapsed: false })
    expect(screen.getByRole('link', { name: 'Welcome' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Settings' })).toBeInTheDocument()
  })

  it('hides the nav labels but keeps the icons reachable when collapsed', () => {
    renderSidebar({ collapsed: true })
    expect(screen.queryByText('Welcome')).not.toBeInTheDocument()
    expect(screen.queryByText('Settings')).not.toBeInTheDocument()
    expect(screen.getAllByRole('link')).toHaveLength(2)
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

  it('highlights Welcome only on an exact match at "/"', () => {
    renderSidebar({}, ['/'])
    expect(screen.getByRole('link', { name: 'Welcome' })).toHaveClass('bg-surface-raised')
    expect(screen.getByRole('link', { name: 'Settings' })).not.toHaveClass('bg-surface-raised')
  })

  it('keeps Settings highlighted on a nested /settings sub-route', () => {
    renderSidebar({}, ['/settings/config'])
    expect(screen.getByRole('link', { name: 'Settings' })).toHaveClass('bg-surface-raised')
    expect(screen.getByRole('link', { name: 'Welcome' })).not.toHaveClass('bg-surface-raised')
  })
})
