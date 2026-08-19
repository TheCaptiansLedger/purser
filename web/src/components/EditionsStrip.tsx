import { Star } from 'lucide-react'
import type { Release } from '../gen/purser/music/v1/release_pb'
import { statusFromProto } from '../hooks/useDiscography'
import { MusicReleaseStatusBadge } from './MusicReleaseStatusBadge'
import { Toggle } from './Toggle'

export interface EditionsStripProps {
  releases: Release[]
  selectedId: string
  onSelect: (id: string) => void
  onToggleMonitored: (release: Release, next: boolean) => void
  // monitoredOverrides carries the optimistic-local-mirror pattern
  // ArtistDetail's handleMonitorToggle established, keyed per release
  // since this strip can hold several in-flight toggles at once — unlike
  // that page's single LibraryEntry-level toggle.
  monitoredOverrides?: Record<string, boolean>
  pendingReleaseId?: string
  // onSetDefault (#677) — reassigning IsDefault, absent on a card that's
  // already the default (nothing to do there). Optional so callers that
  // still want the read-only star (existing tests) keep working unchanged.
  onSetDefault?: (release: Release) => void
  settingDefaultReleaseId?: string
}

// EditionsStrip — Album Detail's Editions strip (#674): a Group's
// MusicRelease rows (see AlbumDetail's ListMusicReleases read), each
// selectable to drive the Tracklist tab's (#676) query — see
// AlbumDetail's selectedReleaseId. Presentation-only, same
// props-in/callbacks-out shape as AlbumCard/PersonCard: no query of its
// own.
//
// IsDefault renders as a Star. Per the style guide's iconography rule (no
// color-only signaling), the filled star is present only on the default
// edition rather than an outline/filled pair on every card, plus a
// visually-hidden label — presence/absence itself communicates the
// state, same precedent #676's disabled-play-icon spec uses.
//
// Reassigning the default edition (#677): a non-default card renders an
// outline-star button in the same spot instead, labeled "Set as default
// edition." It stops propagation (same reason the Monitored toggle does)
// and disables itself while its own release is the one mid-request —
// settingDefaultReleaseId mirrors pendingReleaseId's per-release keying,
// since this strip can have only one Set-default request in flight at a
// time (AlbumDetail serializes the two UpdateMusicRelease calls) but
// still needs to know which card that is.
//
// The Monitored toggle sits inside each card but stops propagation of its
// click so tapping it never also re-selects the edition underneath it.
export function EditionsStrip({
  releases,
  selectedId,
  onSelect,
  onToggleMonitored,
  monitoredOverrides = {},
  pendingReleaseId,
  onSetDefault,
  settingDefaultReleaseId,
}: EditionsStripProps) {
  if (releases.length === 0) {
    return null
  }

  return (
    <div role="tablist" aria-label="Editions" className="flex gap-3 overflow-x-auto pb-1">
      {releases.map(release => {
        const isSelected = release.id === selectedId
        const status = statusFromProto(release.status)
        const isMonitored = monitoredOverrides[release.id] ?? release.monitored

        return (
          <div
            key={release.id}
            role="tab"
            aria-selected={isSelected}
            tabIndex={0}
            onClick={() => onSelect(release.id)}
            onKeyDown={event => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault()
                onSelect(release.id)
              }
            }}
            className={[
              'flex w-56 shrink-0 cursor-pointer flex-col gap-2 rounded-xl border p-3',
              isSelected ? 'border-accent-system bg-surface-raised' : 'border-border hover:bg-surface-raised',
            ].join(' ')}
          >
            <div className="flex items-start justify-between gap-2">
              <span className="truncate text-body font-medium text-text">{release.title}</span>
              {release.isDefault && (
                <span className="flex shrink-0 items-center gap-1 text-status-warning">
                  <Star size={12} className="fill-status-warning" aria-hidden="true" />
                  <span className="sr-only">Default edition</span>
                </span>
              )}

              {!release.isDefault && onSetDefault && (
                <button
                  type="button"
                  aria-label={`Set ${release.title} as default edition`}
                  title="Set as default edition"
                  disabled={settingDefaultReleaseId === release.id}
                  onClick={event => {
                    event.stopPropagation()
                    onSetDefault(release)
                  }}
                  className="flex shrink-0 items-center text-text-secondary hover:text-status-warning disabled:opacity-50"
                >
                  <Star size={12} aria-hidden="true" />
                </button>
              )}
            </div>

            <div className="flex items-center justify-between gap-2 text-label text-text-secondary">
              <span className="truncate">{release.format || 'Unknown format'}</span>
              {status && <MusicReleaseStatusBadge status={status} />}
            </div>

            <div onClick={event => event.stopPropagation()}>
              <Toggle
                label="Monitored"
                checked={isMonitored}
                onChange={next => onToggleMonitored(release, next)}
                disabled={pendingReleaseId === release.id}
              />
            </div>
          </div>
        )
      })}
    </div>
  )
}
