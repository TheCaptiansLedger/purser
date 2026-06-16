import { useState } from 'react'
import { Link } from 'react-router-dom'
import { User } from 'lucide-react'
import type { Person } from '../../types'

interface Props {
  person: Person
  href: string
  accent?: string
}

export function PersonCard({ person, href, accent }: Props) {
  const [imgFailed, setImgFailed] = useState(false)
  const showImg = !!person.imageUrl && !imgFailed

  return (
    <Link to={href} className="group flex flex-col gap-2 cursor-pointer">
      {/* Portrait — 2:3 */}
      <div className="relative rounded-xl overflow-hidden bg-white/4 border border-white/5 group-hover:border-white/15 transition-all duration-200 group-hover:scale-[1.02]"
        style={{ aspectRatio: '2/3' }}>
        {showImg ? (
          <img
            src={person.imageUrl}
            alt={person.name}
            className="w-full h-full object-cover object-top"
            loading="lazy"
            onError={() => setImgFailed(true)}
          />
        ) : (
          <div className="w-full h-full flex items-end justify-center pb-6">
            <User size={48} className="text-white/10" strokeWidth={1} />
          </div>
        )}
        {/* Hover glow */}
        <div
          className="absolute inset-0 opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none rounded-xl"
          style={{ boxShadow: accent ? `inset 0 0 0 1.5px ${accent}55` : undefined }}
        />
        {/* Bottom gradient */}
        <div className="absolute bottom-0 inset-x-0 h-20 bg-gradient-to-t from-black/60 to-transparent pointer-events-none" />
      </div>

      {/* Info */}
      <div className="px-0.5">
        <p className="text-sm font-medium text-white/90 truncate leading-tight">{person.name}</p>
        {person.aliases.length > 0 && (
          <p className="text-xs text-white/35 truncate mt-0.5">
            {person.aliases.slice(0, 2).join(', ')}
          </p>
        )}
      </div>
    </Link>
  )
}
