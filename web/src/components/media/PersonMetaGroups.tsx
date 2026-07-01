import type { ReactNode } from 'react'
import { CountryChip } from '../ui/CountryChip'
import { fmtDate } from '../ui/Runtime'

interface MetaRowProps {
  label: string
  value?: ReactNode
}

function MetaRow({ label, value }: MetaRowProps) {
  if (value === undefined || value === null || value === '') return null
  return (
    <div className="py-3 border-b border-white/5">
      <p className="text-xs text-white/35 mb-0.5">{label}</p>
      <p className="text-sm text-white/75">{value}</p>
    </div>
  )
}

function fmtGender(g: string): string {
  switch (g) {
    case 'non_binary': return 'Non-Binary'
    case 'transgender_male': return 'Trans Male'
    case 'transgender_female': return 'Trans Female'
    case 'unknown': return 'Unknown'
    default: return g.charAt(0).toUpperCase() + g.slice(1)
  }
}

interface RowDef {
  key: string
  label: string
  render?: (value: unknown) => ReactNode
}

interface GroupDef {
  title: string
  rows: RowDef[]
}

const GROUPS: GroupDef[] = [
  {
    title: 'Physical',
    rows: [
      { key: 'height', label: 'Height' },
      { key: 'weight', label: 'Weight' },
      { key: 'measurements', label: 'Measurements' },
      { key: 'cup_size', label: 'Cup Size' },
      { key: 'hair_color', label: 'Hair' },
      { key: 'eye_color', label: 'Eyes' },
    ],
  },
  {
    title: 'Background',
    rows: [
      { key: 'type', label: 'Type' },
      { key: 'gender', label: 'Gender', render: v => fmtGender(String(v)) },
      { key: 'area', label: 'Area' },
      { key: 'begin_area', label: 'Origin' },
      { key: 'end_area', label: 'Ended In' },
      { key: 'birthdate', label: 'Born', render: v => fmtDate(String(v)) },
      { key: 'deathday', label: 'Died', render: v => fmtDate(String(v)) },
      { key: 'place_of_birth', label: 'Birthplace' },
      { key: 'nationality', label: 'Nationality', render: v => <CountryChip value={String(v)} /> },
      { key: 'ethnicity', label: 'Ethnicity' },
      { key: 'known_for', label: 'Known For' },
    ],
  },
  {
    title: 'Career',
    rows: [
      { key: 'career_start', label: 'Career Start' },
      { key: 'career_end', label: 'Career End' },
      { key: 'breast_type', label: 'Breast Type' },
    ],
  },
  {
    title: 'Dates',
    rows: [
      { key: 'begin_date', label: 'Born / Formed' },
      { key: 'end_date', label: 'Died / Disbanded' },
      { key: 'ended', label: 'Ended', render: v => (v === true ? 'Yes' : null) },
    ],
  },
  {
    title: 'Identifiers',
    rows: [
      { key: 'ipi', label: 'IPI' },
      { key: 'disambiguation', label: 'Note' },
      { key: 'homepage', label: 'Website' },
    ],
  },
  {
    title: 'Notes',
    rows: [
      { key: 'tattoos', label: 'Tattoos' },
      { key: 'piercings', label: 'Piercings' },
    ],
  },
]

interface Props {
  metadata?: Record<string, unknown>
}

export function PersonMetaGroups({ metadata }: Props) {
  if (!metadata || Object.keys(metadata).length === 0) return null

  const visibleGroups = GROUPS.map(group => ({
    ...group,
    rows: group.rows.filter(row => {
      const v = metadata[row.key]
      if (v === undefined || v === null || v === '') return false
      if (row.key === 'ended' && v === false) return false
      return true
    }),
  })).filter(group => group.rows.length > 0)

  if (visibleGroups.length === 0) return null

  return (
    <div className="mt-4 pt-3 border-t border-white/5">
      {visibleGroups.map((group, gi) => (
        <div key={group.title} className={gi > 0 ? 'mt-4' : undefined}>
          <p className="text-xs text-white/30 uppercase tracking-wider mb-1">{group.title}</p>
          {group.rows.map(row => {
            const v = metadata[row.key]
            const rendered = row.render ? row.render(v) : String(v)
            return <MetaRow key={row.key} label={row.label} value={rendered} />
          })}
        </div>
      ))}
    </div>
  )
}
