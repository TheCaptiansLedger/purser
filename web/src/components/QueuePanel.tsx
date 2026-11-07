import { X } from 'lucide-react'
import { DragReorderList } from './DragReorderList'
import type { PlayerTrack } from '../lib/playerReducer'

export interface QueuePanelProps {
  queue: PlayerTrack[]
  onReorder: (tracks: PlayerTrack[]) => void
  onRemove: (trackId: string) => void
}

// QueuePanel is #751's "see, reorder, and remove upcoming tracks" half —
// built on DragReorderList (drag + keyboard reorder, already generic)
// rather than a second reorder implementation. onReorder receives the
// whole reordered array, which NowPlayingBar feeds straight into
// playerReducer's existing SET_QUEUE action — no new reducer action
// needed for this.
export function QueuePanel({ queue, onReorder, onRemove }: QueuePanelProps) {
  return (
    <div className="flex w-72 flex-col gap-2 rounded-lg border border-border bg-surface-raised p-2 shadow-xl">
      <h4 className="px-1 text-label font-medium text-text-secondary">Up next</h4>

      {queue.length === 0 ? (
        <p className="px-1 py-2 text-label text-text-secondary">Queue is empty.</p>
      ) : (
        <DragReorderList
          items={queue}
          getKey={track => track.id}
          getLabel={track => track.title}
          ariaLabel="Queue"
          onReorder={onReorder}
          renderItem={track => (
            <div className="flex min-w-0 items-center justify-between gap-2">
              <span className="min-w-0 truncate text-body text-text">
                {track.title}
                {track.artist ? ` — ${track.artist}` : ''}
              </span>
              <button
                type="button"
                onClick={() => onRemove(track.id)}
                aria-label={`Remove ${track.title} from queue`}
                title="Remove from queue"
                className="flex h-6 w-6 shrink-0 items-center justify-center rounded text-text-secondary hover:bg-surface hover:text-status-failure"
              >
                <X size={12} />
              </button>
            </div>
          )}
        />
      )}
    </div>
  )
}
