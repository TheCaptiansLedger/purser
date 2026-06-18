import { useState } from 'react'
import { Link } from 'react-router-dom'
import { ImageIcon, Clock } from 'lucide-react'
import { ItemStatusPopover } from './ItemStatusPopover'
import type { Item } from '../../types'

interface Props {
  item: Item
  href: string
  aspect?: '2/3' | '16/9'
  accent?: string
  showPeople?: boolean
  alwaysShowStatus?: boolean
}

function fmtRuntime(secs: number) {
  if (!secs) return null
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  return h > 0 ? `${h}h ${m}m` : `${m}m`
}

function fmtDate(d?: string) {
  if (!d) return null
  return new Date(d).getFullYear()
}

export function ItemCard({ item, href, aspect = '2/3', accent, showPeople = false, alwaysShowStatus = false }: Props) {
  const [imgFailed, setImgFailed] = useState(false)
  const showImg = !!item.coverUrl && !imgFailed
  const runtime = fmtRuntime(item.runtimeSeconds)
  const year = fmtDate(item.date)
  const performers = item.people.slice(0, 3).map(p => p.person?.name).filter(Boolean)

  return (
    <Link to={href} className="group flex flex-col gap-2 cursor-pointer">
      <div
        className="relative rounded-xl overflow-hidden bg-white/4 border border-white/5 group-hover:border-white/15 transition-all duration-200 group-hover:scale-[1.02]"
        style={{ aspectRatio: aspect }}
      >
        {showImg ? (
          <img
            src={item.coverUrl}
            alt={item.title}
            className="w-full h-full object-cover"
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
        ) : (
          <div className="w-full h-full flex items-center justify-center">
            <ImageIcon size={36} className="text-white/10" strokeWidth={1} />
          </div>
        )}
        {/* Hover ring */}
        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none rounded-xl"
          style={{ boxShadow: accent ? `inset 0 0 0 1.5px ${accent}55` : undefined }}
        />
        {/* Bottom gradient */}
        <div className="absolute bottom-0 inset-x-0 h-24 bg-gradient-to-t from-black/80 to-transparent pointer-events-none" />
        {/* Runtime badge */}
        {runtime && (
          <div className="absolute bottom-2 right-2">
            <span className="flex items-center gap-1 text-xs text-white/60 bg-black/50 backdrop-blur-sm px-2 py-0.5 rounded-md">
              <Clock size={11} />
              {runtime}
            </span>
          </div>
        )}
        {/* Status overlay — always visible or revealed on hover */}
        <div className={[
          'absolute bottom-2 left-2 transition-opacity duration-200',
          alwaysShowStatus ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
        ].join(' ')}>
          <ItemStatusPopover item={item} />
        </div>
      </div>

      <div className="px-0.5">
        <p className="text-sm font-medium text-white/90 truncate leading-tight">{item.title || item.sequence}</p>
        <div className="flex items-center gap-1.5 mt-0.5 min-w-0">
          {year && <span className="text-xs text-white/35 shrink-0">{year}</span>}
          {showPeople && performers.length > 0 && (
            <>
              {year && <span className="text-white/20 text-xs">·</span>}
              <span className="text-xs text-white/35 truncate">{performers.join(', ')}</span>
            </>
          )}
        </div>
      </div>
    </Link>
  )
}
