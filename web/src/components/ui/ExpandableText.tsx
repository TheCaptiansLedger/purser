import { useState, useRef, useLayoutEffect } from 'react'
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
  textClassName?: string
}

export function ExpandableText({ text, maxLines = 4, textClassName }: Props) {
  const [expanded, setExpanded] = useState(false)
  const [isClamped, setIsClamped] = useState(false)
  const ref = useRef<HTMLParagraphElement>(null)

  useLayoutEffect(() => {
    if (expanded) return
    const el = ref.current
    if (!el) return
    setIsClamped(el.scrollHeight > el.clientHeight)
  }, [text, maxLines, expanded])

  return (
    <div>
      <p
        ref={ref}
        className={textClassName ?? 'text-sm text-white/60 leading-relaxed max-w-3xl'}
        style={expandableTextStyle(expanded, maxLines)}
      >
        {text}
      </p>
      {(isClamped || expanded) && (
        <button
          className="mt-2 text-xs text-white/40 hover:text-white/70 transition-colors"
          onClick={() => setExpanded(e => !e)}
        >
          {expanded ? 'Show less' : 'Show more'}
        </button>
      )}
    </div>
  )
}
