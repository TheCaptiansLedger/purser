import { ChevronLeft, ChevronRight } from 'lucide-react'

interface Props {
  total: number
  limit: number
  offset: number
  onChange: (offset: number) => void
  accent?: string
}

export function Pagination({ total, limit, offset, onChange, accent }: Props) {
  if (total <= limit) return null

  const page = Math.floor(offset / limit)
  const pages = Math.ceil(total / limit)

  const go = (p: number) => onChange(p * limit)

  const visiblePages = Array.from({ length: Math.min(7, pages) }, (_, i) => {
    if (pages <= 7) return i
    if (page <= 3) return i
    if (page >= pages - 4) return pages - 7 + i
    return page - 3 + i
  })

  return (
    <div className="flex items-center justify-center gap-1 py-6">
      <button
        onClick={() => go(page - 1)}
        disabled={page === 0}
        className="flex items-center justify-center w-8 h-8 rounded-lg text-white/40 hover:text-white/70 hover:bg-white/5 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
      >
        <ChevronLeft size={16} />
      </button>

      {visiblePages[0] > 0 && (
        <>
          <PageBtn n={0} current={page} onClick={go} accent={accent} />
          {visiblePages[0] > 1 && <span className="px-1 text-white/20 text-sm">…</span>}
        </>
      )}

      {visiblePages.map(n => (
        <PageBtn key={n} n={n} current={page} onClick={go} accent={accent} />
      ))}

      {visiblePages[visiblePages.length - 1] < pages - 1 && (
        <>
          {visiblePages[visiblePages.length - 1] < pages - 2 && <span className="px-1 text-white/20 text-sm">…</span>}
          <PageBtn n={pages - 1} current={page} onClick={go} accent={accent} />
        </>
      )}

      <button
        onClick={() => go(page + 1)}
        disabled={page >= pages - 1}
        className="flex items-center justify-center w-8 h-8 rounded-lg text-white/40 hover:text-white/70 hover:bg-white/5 disabled:opacity-30 disabled:cursor-not-allowed transition-all"
      >
        <ChevronRight size={16} />
      </button>
    </div>
  )
}

function PageBtn({ n, current, onClick, accent }: { n: number; current: number; onClick: (n: number) => void; accent?: string }) {
  const active = n === current
  return (
    <button
      onClick={() => onClick(n)}
      className={['flex items-center justify-center w-8 h-8 rounded-lg text-sm font-medium transition-all', active ? 'text-white' : 'text-white/40 hover:text-white/70 hover:bg-white/5'].join(' ')}
      style={active ? { background: accent ? accent + '33' : 'rgba(255,255,255,0.12)', color: accent ?? 'white' } : {}}
    >
      {n + 1}
    </button>
  )
}
