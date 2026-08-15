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
  it('renders a locked key read-only, by its human label, with its lock reason and no input', () => {
    const settings = [
      baseSetting({ key: 'database.driver', value: '"badger"', locked: true, lockReason: 'bootstrap' }),
    ]
    render(<SettingsCard category="database" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    expect(screen.getByText('Driver')).toBeInTheDocument()
    expect(screen.getByText('badger')).toBeInTheDocument()
    expect(screen.getByText('Requires restart')).toBeInTheDocument()
    expect(screen.queryByText('database.driver')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Driver')).not.toBeInTheDocument()
  })

  it('edits an unlocked toggle by its human label and batches only the dirty key on Save', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const settings = [baseSetting({ key: 'modules.movies.enabled', value: 'false' })]
    render(<SettingsCard category="movies" settings={settings} onSave={onSave} onReset={vi.fn()} saving={false} resettingKey={null} />)

    fireEvent.click(screen.getByRole('switch', { name: 'Enabled' }))
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

  it('shows a unit hint alongside a numeric field with one registered', () => {
    const settings = [baseSetting({ key: 'pipeline.confidence_threshold', value: '0.8' })]
    render(<SettingsCard category="pipeline" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    expect(screen.getByLabelText('Match Confidence Threshold (0.0–1.0)')).toBeInTheDocument()
  })

  it('never pre-fills a set secret with its masked placeholder, and labels it by product name', () => {
    const settings = [baseSetting({ key: 'sources.stashdb.api_key', value: '********', secret: true })]
    render(<SettingsCard category="afterdark" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    const input = screen.getByLabelText<HTMLInputElement>('StashDB API Key')
    expect(input.value).toBe('')
    expect(input.placeholder).toBe('Set — enter a new value to replace')
  })

  it('renders an info popover trigger next to a rename-template field, none for an ordinary field', () => {
    const settings = [
      baseSetting({ key: 'pipeline.organize.adult.template', value: '"{{.SceneTitle}}"' }),
      baseSetting({ key: 'media.path' }),
    ]
    render(<SettingsCard category="afterdark" settings={settings} onSave={vi.fn()} onReset={vi.fn()} saving={false} resettingKey={null} />)

    expect(screen.getByRole('button', { name: 'Show available fields for Rename Template' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Show available fields for Media Path/ })).not.toBeInTheDocument()
  })
})
