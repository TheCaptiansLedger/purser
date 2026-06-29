import { useState } from 'react'
import type { CSSProperties } from 'react'

export function expandableTextStyle(expanded: boolean, maxLines: number): CSSProperties | undefined {
  if (expanded) return undefined
  return {
    overflow: 'hidden',
    display: '-webkit-box',
    WebkitLineClamp: maxLines,
    WebkitBoxOrient: 'vertical',
  }
}

interface Props {
  text: string
  maxLines?: number
}

export function ExpandableText({ text, maxLines = 4 }: Props) {
  const [expanded, setExpanded] = useState(false)

  return (
    <div>
      <p
        className="text-sm text-white/60 leading-relaxed max-w-3xl"
        style={expandableTextStyle(expanded, maxLines)}
      >
        {text}
      </p>
      <button
        className="mt-2 text-xs text-white/40 hover:text-white/70 transition-colors"
        onClick={() => setExpanded(e => !e)}
      >
        {expanded ? 'Show less' : 'Show more'}
      </button>
    </div>
  )
}
