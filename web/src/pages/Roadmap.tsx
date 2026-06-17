import { useQuery } from '@tanstack/react-query'
import { ExternalLink, RefreshCw, AlertCircle, CheckCircle2, Lightbulb, Wrench, Ban } from 'lucide-react'

const GITHUB_REPO = 'TheCaptiansLedger/purser'
const GITHUB_API = `https://api.github.com/repos/${GITHUB_REPO}`

interface GHLabel { id: number; name: string; color: string }
interface GHUser  { login: string; avatar_url: string }
interface GHIssue {
  id: number
  number: number
  title: string
  html_url: string
  labels: GHLabel[]
  user: GHUser
  comments: number
  updated_at: string
  closed_at: string | null
  state: 'open' | 'closed'
}

async function fetchIssues(state: 'open' | 'closed'): Promise<GHIssue[]> {
  const url = state === 'closed'
    ? `${GITHUB_API}/issues?state=closed&per_page=100`
    : `${GITHUB_API}/issues?state=open&per_page=100`
  const res = await fetch(url, { headers: { Accept: 'application/vnd.github.v3+json' } })
  if (res.status === 403) throw new Error('GitHub API rate limited — try again in a minute.')
  if (!res.ok) throw new Error('Failed to fetch issues from GitHub.')
  return res.json()
}

const COLUMNS = [
  { status: 'ready',       label: 'Planned',     icon: Lightbulb,    accent: '#6366f1' },
  { status: 'in-progress', label: 'In Progress',  icon: Wrench,       accent: '#f59e0b' },
  { status: 'blocked',     label: 'Blocked',      icon: Ban,          accent: '#ef4444' },
]

function hasStatus(issue: GHIssue, status: string) {
  return issue.labels.some(l => l.name === `status: ${status}`)
}

function getQuarterKey(dateStr: string) {
  const d = new Date(dateStr)
  return `${d.getFullYear()}-Q${Math.floor(d.getMonth() / 3) + 1}`
}

function displayLabels(labels: GHLabel[]) {
  return labels.filter(l => !l.name.startsWith('status:')).slice(0, 3)
}

function relativeDate(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.floor(hrs / 24)
  if (days === 1) return 'yesterday'
  if (days < 7) return `${days}d ago`
  if (days < 30) return `${Math.floor(days / 7)}w ago`
  return new Date(dateStr).toLocaleDateString()
}

function shortDate(dateStr: string) {
  return new Date(dateStr).toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

function IssueCard({ issue, shipped = false }: { issue: GHIssue; shipped?: boolean }) {
  const labels = displayLabels(issue.labels)
  return (
    <a
      href={issue.html_url}
      target="_blank"
      rel="noopener noreferrer"
      className="group flex flex-col gap-2.5 p-3.5 rounded-xl border border-white/6 bg-white/3 hover:bg-white/6 hover:border-white/12 transition-all duration-150"
    >
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0">
          <img src={issue.user.avatar_url} alt={issue.user.login} className="w-5 h-5 rounded-full shrink-0 opacity-60" />
          <span className="text-[11px] text-white/35 shrink-0">#{issue.number}</span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {issue.comments > 0 && (
            <span className="text-[11px] text-white/30">{issue.comments}</span>
          )}
          <ExternalLink size={12} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
        </div>
      </div>

      <p className="text-sm text-white/80 leading-snug line-clamp-2">{issue.title}</p>

      <div className="flex items-center justify-between gap-2 mt-0.5">
        {shipped && issue.closed_at ? (
          <span className="flex items-center gap-1 text-[11px] text-emerald-400/70">
            <CheckCircle2 size={11} />
            {shortDate(issue.closed_at)}
          </span>
        ) : (
          <span className="text-[11px] text-white/25">{relativeDate(issue.updated_at)}</span>
        )}
        {labels.length > 0 && (
          <div className="flex items-center gap-1 flex-wrap justify-end">
            {labels.map(l => (
              <span
                key={l.id}
                className="px-1.5 py-0.5 rounded text-[10px] font-medium leading-none"
                style={{ background: `#${l.color}22`, color: `#${l.color}`, border: `1px solid #${l.color}40` }}
              >
                {l.name}
              </span>
            ))}
          </div>
        )}
      </div>
    </a>
  )
}

function KanbanColumn({ label, icon: Icon, accent, issues }: {
  status: string; label: string; icon: React.ElementType; accent: string; issues: GHIssue[]
}) {
  return (
    <div className="flex flex-col gap-3 min-w-0">
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center gap-2">
          <Icon size={14} style={{ color: accent }} />
          <span className="text-sm font-medium text-white/70">{label}</span>
        </div>
        <span
          className="text-xs font-semibold px-2 py-0.5 rounded-full"
          style={{ background: `${accent}22`, color: accent }}
        >
          {issues.length}
        </span>
      </div>
      <div className="flex flex-col gap-2">
        {issues.length === 0 ? (
          <div className="px-3 py-6 rounded-xl border border-white/4 text-center text-xs text-white/20">
            No items
          </div>
        ) : (
          issues.map(i => <IssueCard key={i.id} issue={i} />)
        )}
      </div>
    </div>
  )
}

export function Roadmap() {
  const open = useQuery<GHIssue[], Error>({
    queryKey: ['github-issues', 'open'],
    queryFn: () => fetchIssues('open'),
    staleTime: 5 * 60 * 1000,
  })

  const closed = useQuery<GHIssue[], Error>({
    queryKey: ['github-issues', 'closed'],
    queryFn: () => fetchIssues('closed'),
    staleTime: 5 * 60 * 1000,
  })

  const openIssues = (open.data ?? []).filter(i => i.labels.some(l => l.name.startsWith('status:')))
  const shippedIssues = closed.data ?? []

  const quarters = Array.from(new Set(
    shippedIssues.filter(i => i.closed_at).map(i => getQuarterKey(i.closed_at!))
  )).sort((a, b) => b.localeCompare(a))

  const isLoading = open.isLoading || closed.isLoading
  const error = open.error?.message ?? closed.error?.message

  function refetch() {
    open.refetch()
    closed.refetch()
  }

  return (
    <div className="px-8 py-10 max-w-7xl">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-semibold text-white">Roadmap</h1>
          <a
            href={`https://github.com/${GITHUB_REPO}/issues`}
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-white/30 hover:text-white/60 transition-colors flex items-center gap-1.5 mt-1"
          >
            {GITHUB_REPO}
            <ExternalLink size={12} />
          </a>
        </div>
        <button
          onClick={refetch}
          disabled={isLoading}
          className="flex items-center gap-2 px-3 py-2 rounded-lg border border-white/8 bg-white/3 hover:bg-white/6 text-white/50 hover:text-white/80 text-sm transition-all duration-150 disabled:opacity-40"
        >
          <RefreshCw size={14} className={isLoading ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {error && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-xl border border-red-500/20 bg-red-500/8 text-red-400 text-sm mb-6">
          <AlertCircle size={15} className="shrink-0" />
          {error}
        </div>
      )}

      {/* Kanban */}
      {isLoading && openIssues.length === 0 ? (
        <div className="grid grid-cols-3 gap-6">
          {COLUMNS.map(col => (
            <div key={col.status} className="flex flex-col gap-3">
              <div className="h-6 w-28 rounded-lg bg-white/5 animate-pulse" />
              {[1, 2, 3].map(n => (
                <div key={n} className="h-24 rounded-xl bg-white/3 animate-pulse" />
              ))}
            </div>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-6">
          {COLUMNS.map(col => (
            <KanbanColumn
              key={col.status}
              {...col}
              issues={openIssues.filter(i => hasStatus(i, col.status))}
            />
          ))}
        </div>
      )}

      {/* Shipped */}
      {(shippedIssues.length > 0 || (!closed.isLoading && shippedIssues.length === 0)) && (
        <div className="mt-12">
          <div className="flex items-center gap-3 mb-6">
            <CheckCircle2 size={16} className="text-emerald-400" />
            <h2 className="text-base font-semibold text-white/70">Shipped</h2>
            <span className="text-xs text-white/30">{shippedIssues.length} features</span>
          </div>

          {shippedIssues.length === 0 ? (
            <div className="text-sm text-white/25 px-1">Nothing shipped yet.</div>
          ) : (
            <div className="flex flex-col gap-8">
              {quarters.map(qKey => {
                const [year, q] = qKey.split('-')
                const items = shippedIssues
                  .filter(i => i.closed_at && getQuarterKey(i.closed_at) === qKey)
                  .sort((a, b) => new Date(b.closed_at!).getTime() - new Date(a.closed_at!).getTime())
                return (
                  <div key={qKey}>
                    <div className="flex items-center gap-3 mb-3">
                      <span className="text-sm font-medium text-white/50">{q} {year}</span>
                      <span className="text-xs text-white/25">{items.length} features</span>
                    </div>
                    <div className="grid grid-cols-3 gap-3">
                      {items.map(i => <IssueCard key={i.id} issue={i} shipped />)}
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
