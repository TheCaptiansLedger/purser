import { fireEvent, render, screen } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'
import { Layout, parseSidebarCollapsed } from './Layout'

function renderLayout() {
  const router = createMemoryRouter(
    [{ element: <Layout />, children: [{ path: '/', element: <p>page content</p> }] }],
    { initialEntries: ['/'] },
  )
  return render(<RouterProvider router={router} />)
}

describe('parseSidebarCollapsed', () => {
  it('is false for null (no stored preference)', () => {
    expect(parseSidebarCollapsed(null)).toBe(false)
  })

  it('is true only for the literal string "true"', () => {
    expect(parseSidebarCollapsed('true')).toBe(true)
    expect(parseSidebarCollapsed('false')).toBe(false)
    expect(parseSidebarCollapsed('garbage')).toBe(false)
  })
})

describe('Layout', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('renders the routed page content inside the shell', () => {
    renderLayout()
    expect(screen.getByText('page content')).toBeInTheDocument()
  })

  it('opens and closes the mobile drawer via the menu button and backdrop', () => {
    renderLayout()
    expect(screen.queryByRole('button', { name: 'Close navigation' })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Open navigation' }))
    expect(screen.getByRole('button', { name: 'Close navigation' })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Close navigation' }))
    expect(screen.queryByRole('button', { name: 'Close navigation' })).not.toBeInTheDocument()
  })
})
