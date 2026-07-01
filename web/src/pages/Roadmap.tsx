import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  ExternalLink,
  RefreshCw,
  AlertCircle,
  CheckCircle2,
  Lightbulb,
  Wrench,
  Ban,
  LayoutGrid,
  Layers,
  Cpu,
  Database,
  Plug,
  Folder,
  Settings,
  HelpCircle,
  Layout,
  Sparkles
} from 'lucide-react'

const GITHUB_REPO = 'TheCaptiansLedger/purser'
const GITHUB_API = `https://api.github.com/repos/${GITHUB_REPO}`

export interface GHLabel { id: number; name: string; color: string }
export interface GHUser  { login: string; avatar_url: string }
export interface GHIssue {
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
  pull_request?: { url: string }
  assignees?: GHUser[]
}

export interface GHRelease {
  id: number
  tag_name: string
  name: string
  html_url: string
  published_at: string
  body: string
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

export function parseIssueRefs(body: string): number[] {
  const matches = body.match(/#(\d+)/g) ?? []
  return [...new Set(matches.map(m => parseInt(m.slice(1), 10)))]
}

async function fetchReleases(): Promise<GHRelease[]> {
  const res = await fetch(`${GITHUB_API}/releases`, {
    headers: { Accept: 'application/vnd.github.v3+json' },
  })
  if (res.status === 403) throw new Error('GitHub API rate limited — try again in a minute.')
  if (!res.ok) throw new Error('Failed to fetch releases from GitHub.')
  return res.json()
}

export function uniqueContributors(
  commits: Array<{ author: { login: string; avatar_url: string } | null }>
): GHUser[] {
  const seen = new Set<string>()
  const result: GHUser[] = []
  for (const c of commits) {
    if (c.author && !seen.has(c.author.login)) {
      seen.add(c.author.login)
      result.push({ login: c.author.login, avatar_url: c.author.avatar_url })
    }
  }
  return result
}

async function fetchReleaseContributors(prevTag: string | null, currentTag: string): Promise<GHUser[]> {
  const url = prevTag
    ? `${GITHUB_API}/compare/${prevTag}...${currentTag}`
    : `${GITHUB_API}/commits?sha=${currentTag}&per_page=100`
  const res = await fetch(url, { headers: { Accept: 'application/vnd.github.v3+json' } })
  if (res.status === 403) throw new Error('GitHub API rate limited — try again in a minute.')
  if (!res.ok) throw new Error('Failed to fetch release contributors.')
  const data = await res.json()
  return uniqueContributors(prevTag ? data.commits : data)
}

const COLUMNS = [
  { status: 'proposed',    label: 'Proposed',     icon: Sparkles,     accent: '#8b5cf6' },
  { status: 'ready',       label: 'Planned',      icon: Lightbulb,    accent: '#6366f1' },
  { status: 'in-progress', label: 'In Progress',  icon: Wrench,       accent: '#f59e0b' },
  { status: 'blocked',     label: 'Blocked',      icon: Ban,          accent: '#ef4444' },
]

export const AREAS = [
  { id: 'ui',       label: 'User Interface',  color: '#ec4899' },
  { id: 'api',      label: 'API & Routing',   color: '#3b82f6' },
  { id: 'db',       label: 'Database',        color: '#10b981' },
  { id: 'adapter',  label: 'Adapters',         color: '#8b5cf6' },
  { id: 'domain',   label: 'Core Domain',     color: '#14b8a6' },
  { id: 'infra',    label: 'Infrastructure & CI', color: '#f59e0b' },
  { id: 'other',    label: 'General & Other',  color: '#6b7280' },
]

const AREA_ICONS: Record<string, React.ElementType> = {
  ui: Layout,
  api: Cpu,
  db: Database,
  adapter: Plug,
  domain: Folder,
  infra: Settings,
  other: HelpCircle,
}

export function hasStatus(issue: GHIssue, status: string) {
  if (status === 'proposed') {
    return (
      !issue.labels.some(l => l.name.startsWith('status:')) ||
      issue.labels.some(l => l.name === 'status: proposed')
    )
  }
  return issue.labels.some(l => l.name === `status: ${status}`)
}

export function getQuarterKey(dateStr: string) {
  const d = new Date(dateStr)
  return `${d.getUTCFullYear()}-Q${Math.floor(d.getUTCMonth() / 3) + 1}`
}

export function sortedQuarterKeys(releases: GHRelease[]): string[] {
  return Array.from(new Set(releases.map(r => getQuarterKey(r.published_at))))
    .sort((a, b) => b.localeCompare(a))
}

export function getIssueArea(issue: GHIssue): string {
  const areaLabel = issue.labels.find(l => l.name.startsWith('area:'))
  if (!areaLabel) return 'other'
  const name = areaLabel.name.replace('area:', '').trim()
  const exists = AREAS.some(a => a.id === name)
  return exists ? name : 'other'
}

function displayLabels(labels: GHLabel[]) {
  return labels.filter(l =>
    !l.name.startsWith('status:') &&
    !l.name.startsWith('area:') &&
    !l.name.startsWith('scope:')
  ).slice(0, 3)
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

function ContributorAvatar({ user, role }: { user: GHUser; role: 'creator' | 'assignee' | 'contributor' }) {
  const imgClass = role === 'creator'
    ? 'border-indigo-500/40 opacity-70 group-hover/avatar:opacity-100'
    : role === 'assignee'
    ? 'border-emerald-500/40 opacity-55 group-hover/avatar:opacity-100 hover:scale-105'
    : 'border-teal-500/40 opacity-55 group-hover/avatar:opacity-100 hover:scale-105'
  const label = role === 'creator' ? 'Creator' : role === 'assignee' ? 'Assignee' : 'Contributor'
  return (
    <div className="relative group/avatar">
      <img
        src={user.avatar_url}
        alt={user.login}
        className={`w-5 h-5 rounded-full border shrink-0 transition-all ${imgClass}`}
      />
      <span className="absolute bottom-full left-1/2 -translate-x-1/2 mb-1.5 hidden group-hover/avatar:block bg-zinc-950 text-[9px] text-white/90 px-1.5 py-0.5 rounded border border-white/10 whitespace-nowrap z-30 shadow-lg pointer-events-none">
        {label}: {user.login}
      </span>
    </div>
  )
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
        <div className="flex items-center gap-1.5 min-w-0">
          <span className="text-[11px] text-white/35 shrink-0 mr-0.5">#{issue.number}</span>
          <div className="flex items-center -space-x-1 shrink-0">
            <ContributorAvatar user={issue.user} role="creator" />
            {(issue.assignees ?? []).map(assignee => (
              <ContributorAvatar key={assignee.login} user={assignee} role="assignee" />
            ))}
          </div>
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

function ReleaseCard({ release, linkedIssues, contributors = [] }: {
  release: GHRelease
  linkedIssues: GHIssue[]
  contributors?: GHUser[]
}) {
  return (
    <a
      href={release.html_url}
      target="_blank"
      rel="noopener noreferrer"
      className="group flex flex-col gap-2.5 p-3.5 rounded-xl border border-white/6 bg-white/3 hover:bg-white/6 hover:border-white/12 transition-all duration-150"
    >
      <div className="flex items-start justify-between gap-2">
        <span className="text-[11px] font-mono text-emerald-400/70 shrink-0">{release.tag_name}</span>
        <ExternalLink size={12} className="text-white/20 group-hover:text-white/50 transition-colors shrink-0" />
      </div>
      <p className="text-sm text-white/80 leading-snug line-clamp-2">{release.name || release.tag_name}</p>
      <div className="flex items-center justify-between gap-2">
        <span className="text-[11px] text-white/25">{shortDate(release.published_at)}</span>
        {contributors.length > 0 && (
          <div className="flex items-center -space-x-1">
            {contributors.map(u => (
              <ContributorAvatar key={u.login} user={u} role="contributor" />
            ))}
          </div>
        )}
      </div>
      {linkedIssues.length > 0 && (
        <div className="mt-1 flex flex-col gap-1 border-t border-white/6 pt-2">
          {linkedIssues.map(i => (
            <a
              key={i.id}
              href={i.html_url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1.5 text-[11px] text-white/45 hover:text-white/75 transition-colors"
              onClick={e => e.stopPropagation()}
            >
              <CheckCircle2 size={10} className="text-emerald-400/60 shrink-0" />
              <span className="text-white/30 shrink-0">#{i.number}</span>
              <span className="truncate">{i.title}</span>
            </a>
          ))}
        </div>
      )}
    </a>
  )
}

function ReleaseCardWithContributors({ release, prevTag, linkedIssues }: {
  release: GHRelease
  prevTag: string | null
  linkedIssues: GHIssue[]
}) {
  const { data: contributors } = useQuery<GHUser[], Error>({
    queryKey: ['release-contributors', release.tag_name],
    queryFn: () => fetchReleaseContributors(prevTag, release.tag_name),
    staleTime: 60 * 60 * 1000,
  })
  return <ReleaseCard release={release} linkedIssues={linkedIssues} contributors={contributors ?? []} />
}

function ShippedColumn({ releases, issueMap }: { releases: GHRelease[]; issueMap: Map<number, GHIssue> }) {
  const quarterKeys = sortedQuarterKeys(releases)

  const oldestFirst = [...releases].sort(
    (a, b) => new Date(a.published_at).getTime() - new Date(b.published_at).getTime()
  )
  const prevTagMap = new Map<string, string | null>(
    oldestFirst.map((r, idx) => [r.tag_name, idx === 0 ? null : oldestFirst[idx - 1].tag_name])
  )

  return (
    <div className="flex flex-col gap-3 flex-1 min-w-64">
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center gap-2">
          <CheckCircle2 size={14} className="text-emerald-400" />
          <span className="text-sm font-medium text-white/70">Shipped</span>
        </div>
        <span
          className="text-xs font-semibold px-2 py-0.5 rounded-full"
          style={{ background: '#10b98122', color: '#10b981' }}
        >
          {releases.length}
        </span>
      </div>
      <div className="flex flex-col">
        {quarterKeys.map((qKey, idx) => {
          const [year, q] = qKey.split('-')
          const items = releases
            .filter(r => getQuarterKey(r.published_at) === qKey)
            .sort((a, b) => new Date(b.published_at).getTime() - new Date(a.published_at).getTime())
          return (
            <div key={qKey}>
              {idx > 0 && <div className="border-t border-white/6 my-4" />}
              <div className="flex items-center gap-2 mb-2 px-1">
                <span className="text-[11px] font-medium text-white/35">{q} {year}</span>
              </div>
              <div className="flex flex-col gap-2">
                {items.map(r => {
                  const refs = parseIssueRefs(r.body ?? '')
                  const linked = refs
                    .map(n => issueMap.get(n))
                    .filter((i): i is GHIssue => !!i && i.labels.some(l => l.name === 'scope: epic'))
                  return (
                    <ReleaseCardWithContributors
                      key={r.id}
                      release={r}
                      prevTag={prevTagMap.get(r.tag_name) ?? null}
                      linkedIssues={linked}
                    />
                  )
                })}
              </div>
            </div>
          )
        })}
      </div>
    </div>
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

  const releases = useQuery<GHRelease[], Error>({
    queryKey: ['github-releases'],
    queryFn: fetchReleases,
    staleTime: 5 * 60 * 1000,
  })

  const [viewMode, setViewMode] = useState<'kanban' | 'swimlanes'>(() => {
    return (localStorage.getItem('purser:roadmap:viewMode') as 'kanban' | 'swimlanes') || 'swimlanes'
  })

  const isVisible = (i: GHIssue) =>
    !i.pull_request &&
    !i.labels.some(l => l.name === 'scope: task') &&
    (i.labels.some(l => l.name === 'scope: epic') || i.labels.some(l => l.name === 'type: bug'))

  const openIssues = (open.data ?? []).filter(isVisible)

  const activeAreas = AREAS.filter(area =>
    openIssues.some(i => getIssueArea(i) === area.id)
  )

  const issueMap = new Map<number, GHIssue>(
    [...(open.data ?? []), ...(closed.data ?? [])].map(i => [i.number, i])
  )

  const isLoading = open.isLoading || closed.isLoading
  const error = open.error?.message ?? closed.error?.message ?? releases.error?.message

  function refetch() {
    open.refetch()
    closed.refetch()
    releases.refetch()
  }

  return (
    <div className="px-8 py-10">
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

        <div className="flex items-center gap-3">
          {/* View Mode Toggle */}
          {!error && !isLoading && openIssues.length > 0 && (
            <div className="flex items-center rounded-lg border border-white/8 bg-white/3 p-0.5 shrink-0">
              <button
                onClick={() => {
                  setViewMode('swimlanes')
                  localStorage.setItem('purser:roadmap:viewMode', 'swimlanes')
                }}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                  viewMode === 'swimlanes'
                    ? 'bg-white/10 text-white shadow-sm'
                    : 'text-white/40 hover:text-white/75'
                }`}
                title="Swimlanes by Area"
              >
                <Layers size={13} />
                Swimlanes
              </button>
              <button
                onClick={() => {
                  setViewMode('kanban')
                  localStorage.setItem('purser:roadmap:viewMode', 'kanban')
                }}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all ${
                  viewMode === 'kanban'
                    ? 'bg-white/10 text-white shadow-sm'
                    : 'text-white/40 hover:text-white/75'
                }`}
                title="Standard Kanban Board"
              >
                <LayoutGrid size={13} />
                Standard
              </button>
            </div>
          )}

          <button
            onClick={refetch}
            disabled={isLoading}
            className="flex items-center gap-2 px-3 py-2 rounded-lg border border-white/8 bg-white/3 hover:bg-white/6 text-white/50 hover:text-white/80 text-sm transition-all duration-150 disabled:opacity-40 shrink-0"
          >
            <RefreshCw size={14} className={isLoading ? 'animate-spin' : ''} />
            Refresh
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-center gap-2 px-4 py-3 rounded-xl border border-red-500/20 bg-red-500/8 text-red-400 text-sm mb-6">
          <AlertCircle size={15} className="shrink-0" />
          {error}
        </div>
      )}

      {/* Board + Shipped flex row */}
      <div className="flex flex-col xl:flex-row gap-6 items-start">
        <div className="flex-[3] min-w-0">
          {isLoading && openIssues.length === 0 ? (
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
              {COLUMNS.map(col => (
                <div key={col.status} className="flex flex-col gap-3">
                  <div className="h-6 w-28 rounded-lg bg-white/5 animate-pulse" />
                  {[1, 2, 3].map(n => (
                    <div key={n} className="h-24 rounded-xl bg-white/3 animate-pulse" />
                  ))}
                </div>
              ))}
            </div>
          ) : openIssues.length === 0 ? (
            <div className="text-center py-12 rounded-xl border border-white/6 bg-white/1 text-white/30 text-sm">
              No active roadmap issues found.
            </div>
          ) : viewMode === 'swimlanes' ? (
            <div className="flex flex-col gap-8">
              {/* Global Column Headers for Swimlanes */}
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6 mb-2 px-1">
                {COLUMNS.map(col => {
                  const Icon = col.icon
                  const count = openIssues.filter(i => hasStatus(i, col.status)).length
                  return (
                    <div key={col.status} className="flex items-center gap-2 pb-2 border-b border-white/4">
                      <Icon size={14} style={{ color: col.accent }} />
                      <span className="text-xs font-semibold text-white/60 uppercase tracking-wider">{col.label}</span>
                      <span
                        className="text-[10px] font-bold px-1.5 py-0.5 rounded-full"
                        style={{ background: `${col.accent}15`, color: col.accent }}
                      >
                        {count}
                      </span>
                    </div>
                  )
                })}
              </div>

              {/* Swimlanes list */}
              <div className="flex flex-col gap-6">
                {activeAreas.map(area => {
                  const areaIssues = openIssues.filter(i => getIssueArea(i) === area.id)
                  const AreaIcon = AREA_ICONS[area.id] || HelpCircle

                  return (
                    <div
                      key={area.id}
                      className="flex flex-col gap-4 p-4 rounded-xl border border-white/6 bg-white/1 hover:border-white/10 transition-colors duration-150"
                    >
                      {/* Swimlane Header */}
                      <div className="flex items-center gap-2.5">
                        <AreaIcon size={16} style={{ color: area.color }} />
                        <span className="text-sm font-semibold text-white/95">{area.label}</span>
                        <span
                          className="text-[10px] px-2 py-0.5 rounded-full font-medium"
                          style={{ background: `${area.color}15`, color: area.color }}
                        >
                          {areaIssues.length} open
                        </span>
                      </div>

                      {/* 4 Columns under this Swimlane */}
                      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
                        {COLUMNS.map(col => {
                          const colIssues = areaIssues.filter(i => hasStatus(i, col.status))
                          return (
                            <div key={col.status} className="flex flex-col gap-2.5 min-w-0">
                              {colIssues.length === 0 ? (
                                <div className="h-full min-h-[4rem] flex items-center justify-center rounded-xl border border-dashed border-white/4 bg-white/[0.01] text-center text-xs text-white/15">
                                  No {col.label.toLowerCase()}
                                </div>
                              ) : (
                                colIssues.map(i => <IssueCard key={i.id} issue={i} />)
                              )}
                            </div>
                          )
                        })}
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          ) : (
            /* Standard Kanban Board */
            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 md:gap-6">
              {COLUMNS.map(col => (
                <KanbanColumn
                  key={col.status}
                  {...col}
                  issues={openIssues.filter(i => hasStatus(i, col.status))}
                />
              ))}
            </div>
          )}
        </div>

        {releases.data && releases.data.length > 0 && (
          <ShippedColumn releases={releases.data} issueMap={issueMap} />
        )}
      </div>
    </div>
  )
}
