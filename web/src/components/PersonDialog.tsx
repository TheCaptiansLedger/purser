import { useMutation } from '@connectrpc/connect-query'
import { timestampDate, timestampFromDate } from '@bufbuild/protobuf/wkt'
import { useState } from 'react'
import { Gender, type Person } from '../gen/purser/domain/v1/person_pb'
import { MonitorMode } from '../gen/purser/domain/v1/common_pb'
import { createPerson, updatePerson } from '../gen/purser/domain/v1/person-PersonService_connectquery'
import { ListInput } from './ListInput'
import { Modal } from './Modal'
import { TextInput } from './TextInput'
import { Toggle } from './Toggle'

const GENDER_OPTIONS: { value: Gender; label: string }[] = [
  { value: Gender.MALE, label: 'Male' },
  { value: Gender.FEMALE, label: 'Female' },
  { value: Gender.TRANSGENDER_MALE, label: 'Transgender Male' },
  { value: Gender.TRANSGENDER_FEMALE, label: 'Transgender Female' },
  { value: Gender.INTERSEX, label: 'Intersex' },
  { value: Gender.NON_BINARY, label: 'Non-binary' },
  { value: Gender.UNKNOWN, label: 'Unknown' },
]

const MONITOR_MODE_OPTIONS: { value: MonitorMode; label: string }[] = [
  { value: MonitorMode.ALL, label: 'All' },
  { value: MonitorMode.FUTURE, label: 'Future' },
  { value: MonitorMode.NONE, label: 'None' },
  { value: MonitorMode.LATEST, label: 'Latest' },
]

interface FormState {
  name: string
  sortName: string
  aliases: string[]
  gender: Gender
  pronouns: string
  birthDate: string
  deathDate: string
  nationality: string
  overview: string
  monitored: boolean
  monitorMode: MonitorMode
}

// FIELD_PATHS pairs each editable field with the proto field-mask path
// applyPersonFieldMask (internal/api/connect/person_convert.go) actually
// switches on — kept as one array so the diffing loop in handleSubmit and
// the mask sent to the server can never drift apart.
const FIELD_PATHS: { key: keyof FormState; path: string }[] = [
  { key: 'name', path: 'name' },
  { key: 'sortName', path: 'sort_name' },
  { key: 'aliases', path: 'aliases' },
  { key: 'gender', path: 'gender' },
  { key: 'pronouns', path: 'pronouns' },
  { key: 'birthDate', path: 'birth_date' },
  { key: 'deathDate', path: 'death_date' },
  { key: 'nationality', path: 'nationality' },
  { key: 'overview', path: 'overview' },
  { key: 'monitored', path: 'monitored' },
  { key: 'monitorMode', path: 'monitor_mode' },
]

function toDateInput(timestamp: Person['birthDate']): string {
  return timestamp ? timestampDate(timestamp).toISOString().slice(0, 10) : ''
}

// initialState seeds a blank form on create (Gender/MonitorMode default to
// the domain's own explicit fallbacks — Unknown/All — rather than an
// unselected value, since GENDER_UNSPECIFIED/MONITOR_MODE_UNSPECIFIED both
// fail domain.Person.Validate()'s oneof check) or pre-fills every field
// UpdatePerson can change from the existing record on edit.
function initialState(person?: Person): FormState {
  if (!person) {
    return {
      name: '',
      sortName: '',
      aliases: [],
      gender: Gender.UNKNOWN,
      pronouns: '',
      birthDate: '',
      deathDate: '',
      nationality: '',
      overview: '',
      monitored: true,
      monitorMode: MonitorMode.ALL,
    }
  }
  return {
    name: person.name,
    sortName: person.sortName,
    aliases: person.aliases,
    gender: person.gender,
    pronouns: person.pronouns,
    birthDate: toDateInput(person.birthDate),
    deathDate: toDateInput(person.deathDate),
    nationality: person.nationality,
    overview: person.overview,
    monitored: person.monitored,
    monitorMode: person.monitorMode,
  }
}

function fieldEqual(key: keyof FormState, a: FormState, b: FormState): boolean {
  if (key === 'aliases') return JSON.stringify(a.aliases) === JSON.stringify(b.aliases)
  return a[key] === b[key]
}

export interface PersonDialogProps {
  mode: 'create' | 'edit'
  // Required in edit mode (the record being edited); ignored in create
  // mode.
  person?: Person
  onClose: () => void
  onSaved: (person: Person) => void
}

// PersonDialog is #663's Add/Edit Person dialog — manual entry only, no
// external-provider search, per that issue's scope note. One form serves
// both modes: create sends a fully-formed Person to CreatePerson (id is
// server-generated, see docs/adr/0020), edit diffs the current form
// against the record it was pre-filled from and sends only the touched
// fields' paths on UpdatePerson's field mask, per ADR 0011's partial-update
// contract.
export function PersonDialog({ mode, person, onClose, onSaved }: PersonDialogProps) {
  const [initial] = useState(() => initialState(mode === 'edit' ? person : undefined))
  const [form, setForm] = useState<FormState>(initial)
  const [nameTouched, setNameTouched] = useState(false)
  const createMutation = useMutation(createPerson)
  const updateMutation = useMutation(updatePerson)
  const mutation = mode === 'create' ? createMutation : updateMutation

  const trimmedName = form.name.trim()
  const nameInvalid = trimmedName === ''

  const changedPaths =
    mode === 'edit' ? FIELD_PATHS.filter(({ key }) => !fieldEqual(key, form, initial)).map(({ path }) => path) : []
  // Save stays clickable while Name is blank so a first submit attempt can
  // reveal the inline error (below) rather than leaving a silently inert
  // button; it's only disabled for states with nothing meaningful to do —
  // an in-flight mutation, or an edit with no touched fields yet.
  const canSubmit = mode === 'create' ? !mutation.isPending : !mutation.isPending && changedPaths.length > 0

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm(prev => ({ ...prev, [key]: value }))
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setNameTouched(true)
    if (!canSubmit || nameInvalid) return

    const wire = {
      name: trimmedName,
      sortName: form.sortName,
      aliases: form.aliases,
      gender: form.gender,
      pronouns: form.pronouns,
      birthDate: form.birthDate ? timestampFromDate(new Date(form.birthDate)) : undefined,
      deathDate: form.deathDate ? timestampFromDate(new Date(form.deathDate)) : undefined,
      nationality: form.nationality,
      overview: form.overview,
      monitored: form.monitored,
      monitorMode: form.monitorMode,
    }

    if (mode === 'create') {
      createMutation.mutate({ person: wire }, { onSuccess: response => response.person && onSaved(response.person) })
      return
    }

    updateMutation.mutate(
      { person: { id: person?.id ?? '', ...wire }, updateMask: { paths: changedPaths } },
      { onSuccess: response => response.person && onSaved(response.person) },
    )
  }

  return (
    <Modal title={mode === 'create' ? 'Add Person' : 'Edit Person'} onClose={onClose}>
      <form onSubmit={handleSubmit} className="flex flex-col gap-4">
        <div className="flex flex-col gap-1">
          <TextInput label="Name" value={form.name} onChange={v => update('name', String(v))} />
          {nameTouched && nameInvalid && (
            <span className="text-label text-status-failure" role="alert">
              Name is required.
            </span>
          )}
        </div>

        <TextInput label="Sort name" value={form.sortName} onChange={v => update('sortName', String(v))} />

        <ListInput label="Aliases" value={form.aliases} onChange={v => update('aliases', v)} />

        <label className="flex flex-col gap-1 text-body text-text">
          <span className="text-label text-text-secondary">Gender</span>
          <select
            value={form.gender}
            onChange={e => update('gender', Number(e.target.value) as Gender)}
            className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          >
            {GENDER_OPTIONS.map(opt => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>

        <TextInput label="Pronouns" value={form.pronouns} onChange={v => update('pronouns', String(v))} />

        <div className="flex gap-3">
          <label className="flex flex-1 flex-col gap-1 text-body text-text">
            <span className="text-label text-text-secondary">Birth date</span>
            <input
              type="date"
              value={form.birthDate}
              onChange={e => update('birthDate', e.target.value)}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            />
          </label>
          <label className="flex flex-1 flex-col gap-1 text-body text-text">
            <span className="text-label text-text-secondary">Death date</span>
            <input
              type="date"
              value={form.deathDate}
              onChange={e => update('deathDate', e.target.value)}
              className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
            />
          </label>
        </div>

        <TextInput label="Nationality" value={form.nationality} onChange={v => update('nationality', String(v))} />

        <label className="flex flex-col gap-1 text-body text-text">
          <span className="text-label text-text-secondary">Overview</span>
          <textarea
            value={form.overview}
            onChange={e => update('overview', e.target.value)}
            rows={3}
            className="px-3 py-2 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          />
        </label>

        <Toggle label="Monitored" checked={form.monitored} onChange={v => update('monitored', v)} />

        <label className="flex flex-col gap-1 text-body text-text">
          <span className="text-label text-text-secondary">Monitor mode</span>
          <select
            value={form.monitorMode}
            onChange={e => update('monitorMode', Number(e.target.value) as MonitorMode)}
            className="h-9 px-3 rounded-lg bg-surface-raised border border-border text-text text-body focus:outline-none focus:border-border-hover"
          >
            {MONITOR_MODE_OPTIONS.map(opt => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
        </label>

        {mutation.isError && (
          <p className="text-body text-status-failure" role="alert">
            Couldn't save this person ({mutation.error.message}).
          </p>
        )}

        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={onClose}
            className="h-9 px-4 rounded-lg bg-surface text-body font-medium text-text-secondary hover:text-text"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={!canSubmit || mutation.isPending}
            className="h-9 px-4 rounded-lg bg-accent-system text-bg text-body font-medium hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {mutation.isPending ? 'Saving…' : 'Save'}
          </button>
        </div>
      </form>
    </Modal>
  )
}
