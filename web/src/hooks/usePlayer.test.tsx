import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { usePlayer } from './usePlayer'
import { PlayerProvider } from '../components/PlayerProvider'

function Consumer() {
  const { state } = usePlayer()
  return <span>{state.isPlaying ? 'playing' : 'idle'}</span>
}

describe('usePlayer', () => {
  it('throws when used outside a PlayerProvider', () => {
    // React logs its own error boundary noise for a thrown render — expected
    // here, not asserted on.
    expect(() => render(<Consumer />)).toThrow('usePlayer must be used within a PlayerProvider')
  })

  it('returns the player context value inside a PlayerProvider', () => {
    render(
      <PlayerProvider>
        <Consumer />
      </PlayerProvider>,
    )
    expect(screen.getByText('idle')).toBeInTheDocument()
  })
})
