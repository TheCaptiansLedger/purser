import { create } from '@bufbuild/protobuf'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ReleaseSchema, ReleaseStatus } from '../gen/purser/music/v1/release_pb'
import { EditionsStrip } from './EditionsStrip'

const stub = create(ReleaseSchema, {
  id: 'rel-1',
  title: '1975 Original Pressing',
  format: 'Vinyl',
  isDefault: false,
  monitored: false,
  status: ReleaseStatus.STUB,
})

const imported = create(ReleaseSchema, {
  id: 'rel-2',
  title: '2024 Remaster',
  format: 'CD',
  isDefault: true,
  monitored: true,
  status: ReleaseStatus.IMPORTED,
})

describe('EditionsStrip', () => {
  it('renders each edition with its format, status badge, and default star', () => {
    render(
      <EditionsStrip
        releases={[stub, imported]}
        selectedId="rel-2"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
      />,
    )

    expect(screen.getByText('1975 Original Pressing')).toBeInTheDocument()
    expect(screen.getByText('Vinyl')).toBeInTheDocument()
    expect(screen.getByText('Stub')).toBeInTheDocument()

    expect(screen.getByText('2024 Remaster')).toBeInTheDocument()
    expect(screen.getByText('CD')).toBeInTheDocument()
    expect(screen.getByText('Imported')).toBeInTheDocument()
    expect(screen.getByText('Default edition')).toBeInTheDocument()
  })

  it('marks only the selected edition as the active tab', () => {
    render(
      <EditionsStrip
        releases={[stub, imported]}
        selectedId="rel-2"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
      />,
    )

    const tabs = screen.getAllByRole('tab')
    expect(tabs[0]).toHaveAttribute('aria-selected', 'false')
    expect(tabs[1]).toHaveAttribute('aria-selected', 'true')
  })

  it('selects an edition on click', () => {
    const onSelect = vi.fn()
    render(
      <EditionsStrip releases={[stub, imported]} selectedId="rel-2" onSelect={onSelect} onToggleMonitored={vi.fn()} />,
    )

    fireEvent.click(screen.getByText('1975 Original Pressing'))
    expect(onSelect).toHaveBeenCalledWith('rel-1')
  })

  it('toggles Monitored without also re-selecting the edition underneath it', () => {
    const onSelect = vi.fn()
    const onToggleMonitored = vi.fn()
    render(
      <EditionsStrip releases={[stub]} selectedId="rel-2" onSelect={onSelect} onToggleMonitored={onToggleMonitored} />,
    )

    fireEvent.click(screen.getByRole('switch', { name: 'Monitored' }))

    expect(onToggleMonitored).toHaveBeenCalledWith(stub, true)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('reflects a pending monitoredOverrides value over the release prop', () => {
    render(
      <EditionsStrip
        releases={[stub]}
        selectedId="rel-1"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
        monitoredOverrides={{ 'rel-1': true }}
      />,
    )

    expect(screen.getByRole('switch', { name: 'Monitored' })).toHaveAttribute('aria-checked', 'true')
  })

  it('disables the Monitored toggle for the edition whose mutation is in flight', () => {
    render(
      <EditionsStrip
        releases={[stub]}
        selectedId="rel-1"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
        pendingReleaseId="rel-1"
      />,
    )

    expect(screen.getByRole('switch', { name: 'Monitored' })).toBeDisabled()
  })

  it('sets a non-default edition as default without also re-selecting it (#677)', () => {
    const onSelect = vi.fn()
    const onSetDefault = vi.fn()
    render(
      <EditionsStrip
        releases={[stub, imported]}
        selectedId="rel-2"
        onSelect={onSelect}
        onToggleMonitored={vi.fn()}
        onSetDefault={onSetDefault}
      />,
    )

    fireEvent.click(screen.getByRole('button', { name: 'Set 1975 Original Pressing as default edition' }))

    expect(onSetDefault).toHaveBeenCalledWith(stub)
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('renders no set-default control on the already-default edition', () => {
    render(
      <EditionsStrip
        releases={[imported]}
        selectedId="rel-2"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
        onSetDefault={vi.fn()}
      />,
    )

    expect(screen.queryByRole('button', { name: /Set .* as default edition/ })).not.toBeInTheDocument()
  })

  it('disables the set-default control for the edition whose reassignment is in flight', () => {
    render(
      <EditionsStrip
        releases={[stub]}
        selectedId="rel-2"
        onSelect={vi.fn()}
        onToggleMonitored={vi.fn()}
        onSetDefault={vi.fn()}
        settingDefaultReleaseId="rel-1"
      />,
    )

    expect(screen.getByRole('button', { name: 'Set 1975 Original Pressing as default edition' })).toBeDisabled()
  })

  it('renders nothing for a Group with zero editions', () => {
    const { container } = render(
      <EditionsStrip releases={[]} selectedId="" onSelect={vi.fn()} onToggleMonitored={vi.fn()} />,
    )

    expect(container).toBeEmptyDOMElement()
  })
})
