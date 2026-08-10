import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Setting } from '../../types'
import { SettingsCard } from './SettingsCard'

function baseSetting(overrides: Partial<Setting>): Setting {
  return {
    key: 'media.path',
    value: '"/data/media"',
    source: 'default',
    locked: false,
    lockReason: 'none',
    secret: false,
    ...overrides,
  }
}

describe('SettingsCard', () => {
  it('renders a locked key read-only with its lock reason, no input', () => {
    const settings = [
      baseSetting({ key: 'database.driver', value: '"badger"', locked: true, lockReason: 'bootstrap' }),
    ]
    render(<SettingsCard category="database" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    expect(screen.getByText('database.driver')).toBeInTheDocument()
    expect(screen.getByText('badger')).toBeInTheDocument()
    expect(screen.getByText('Requires restart')).toBeInTheDocument()
    expect(screen.queryByLabelText('database.driver')).not.toBeInTheDocument()
  })

  it('edits an unlocked toggle and batches only the dirty key on Save', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const settings = [baseSetting({ key: 'modules.movies.enabled', value: 'false' })]
    render(<SettingsCard category="modules" settings={settings} onSave={onSave} onReset={vi.fn()} saving={false} resettingKey={null} />)

    fireEvent.click(screen.getByRole('switch', { name: 'modules.movies.enabled' }))
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    expect(onSave).toHaveBeenCalledWith({ 'modules.movies.enabled': 'true' })
  })

  it('shows Reset only for a DB-overridden unlocked key, and calls onReset with its key', () => {
    const onReset = vi.fn()
    const settings = [
      baseSetting({ key: 'media.path', source: 'db' }),
      baseSetting({ key: 'pipeline.confidence_threshold', value: '0.8', source: 'default' }),
    ]
    render(<SettingsCard category="media" settings={settings} onSave={vi.fn()} onReset={onReset} saving={false} resettingKey={null} />)

    const resetButtons = screen.getAllByRole('button', { name: 'Reset' })
    expect(resetButtons).toHaveLength(1)

    fireEvent.click(resetButtons[0])
    expect(onReset).toHaveBeenCalledWith('media.path')
  })

  it('never pre-fills a set secret with its masked placeholder', () => {
    const settings = [baseSetting({ key: 'sources.stashdb.api_key', value: '********', secret: true })]
    render(<SettingsCard category="sources" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    const input = screen.getByLabelText<HTMLInputElement>('sources.stashdb.api_key')
    expect(input.value).toBe('')
    expect(input.placeholder).toBe('Set — enter a new value to replace')
  })
})
