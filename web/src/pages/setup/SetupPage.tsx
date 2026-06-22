import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { StepIndicator } from '../../components/ui/StepIndicator'
import { useAppConfig, usePatchConfig } from '../../api/config'
import type { PatchConfigRequest } from '../../api/config'
import { useCompleteSetup, useVerifySource } from '../../api/setup'

const TOTAL_STEPS = 5

export type ModuleKey = 'movies' | 'tv' | 'music' | 'afterdark' | 'books'

export interface ModuleState {
  movies: boolean
  tv: boolean
  music: boolean
  afterdark: boolean
  books: boolean
}

export const DEFAULT_MODULES: ModuleState = {
  movies:    true,
  tv:        true,
  music:     true,
  afterdark: false,
  books:     false,
}

interface ModuleMeta {
  label: string
  description: string
  icon: string
}

export const MODULE_META: Record<ModuleKey, ModuleMeta> = {
  movies: {
    label:       'Movies',
    description: 'Track your film collection with rich metadata: cast, crew, genres, ratings, and artwork from TMDB.',
    icon:        '🎬',
  },
  tv: {
    label:       'TV Shows',
    description: 'Manage series, seasons, and episodes with full cast information and air-date tracking via TVDB.',
    icon:        '📺',
  },
  music: {
    label:       'Music',
    description: 'Organise artists and albums with MusicBrainz IDs, cover art, and release metadata.',
    icon:        '🎵',
  },
  afterdark: {
    label:       'After Dark',
    description: 'Adult content library with studio, network, scene, and performer metadata from StashDB and TPDB.',
    icon:        '🌙',
  },
  books: {
    label:       'Books',
    description: 'Catalogue your reading list with author profiles, series grouping, and cover art.',
    icon:        '📚',
  },
}

// ── Source definitions ────────────────────────────────────────────────────────

export type SourceID = 'tmdb' | 'tvdb' | 'mbz' | 'audiodb' | 'fanart' | 'stashdb'

export interface SourceDef {
  id: SourceID
  label: string
  requiresApiKey: boolean
  hasEndpointUrl: boolean
  defaultEndpointUrl?: string
}

export const SOURCE_DEFS: Record<SourceID, SourceDef> = {
  tmdb: {
    id: 'tmdb',
    label: 'TMDB',
    requiresApiKey: true,
    hasEndpointUrl: false,
  },
  tvdb: {
    id: 'tvdb',
    label: 'TVDB',
    requiresApiKey: true,
    hasEndpointUrl: false,
  },
  mbz: {
    id: 'mbz',
    label: 'MusicBrainz',
    requiresApiKey: false,
    hasEndpointUrl: false,
  },
  audiodb: {
    id: 'audiodb',
    label: 'TheAudioDB',
    requiresApiKey: true,
    hasEndpointUrl: false,
  },
  fanart: {
    id: 'fanart',
    label: 'fanart.tv',
    requiresApiKey: true,
    hasEndpointUrl: false,
  },
  stashdb: {
    id: 'stashdb',
    label: 'StashDB',
    requiresApiKey: true,
    hasEndpointUrl: true,
    defaultEndpointUrl: 'https://stashdb.org/graphql',
  },
}

// Maps wizard source IDs to their Go config key prefix.
export const SOURCE_CONFIG_PREFIX: Record<SourceID, string> = {
  tmdb:    'sources.tmdb',
  tvdb:    'sources.tvdb',
  mbz:     'sources.musicbrainz',
  audiodb: 'sources.theaudiodb',
  fanart:  'sources.fanart',
  stashdb: 'sources.stashdb',
}

// Maps wizard source IDs to the Go PATCH request key for the sources object.
const SOURCE_PATCH_KEY: Record<SourceID, keyof NonNullable<PatchConfigRequest['sources']>> = {
  tmdb:    'tmdb',
  tvdb:    'tvdb',
  mbz:     'musicbrainz',
  audiodb: 'theaudiodb',
  fanart:  'fanart',
  stashdb: 'stashdb',
}

// Converts a dotted config key to the corresponding PURSER_ env var name.
// e.g. "sources.tmdb.api_key" → "PURSER_SOURCES_TMDB_API_KEY"
export function configKeyToEnvVar(key: string): string {
  return 'PURSER_' + key.toUpperCase().replace(/\./g, '_')
}

export const MODULE_SOURCES: Record<ModuleKey, SourceID[]> = {
  movies:    ['tmdb'],
  tv:        ['tvdb', 'tmdb'],
  music:     ['mbz', 'audiodb', 'fanart'],
  afterdark: ['stashdb'],
  books:     [],
}

export function sourcesForModules(modules: ModuleState): SourceDef[] {
  const seen = new Set<SourceID>()
  const result: SourceDef[] = []
  for (const key of Object.keys(MODULE_SOURCES) as ModuleKey[]) {
    if (!modules[key]) continue
    for (const id of MODULE_SOURCES[key]) {
      if (!seen.has(id)) {
        seen.add(id)
        result.push(SOURCE_DEFS[id])
      }
    }
  }
  return result
}

export function canProceedFromSources(
  defs: SourceDef[],
  statuses: Record<string, 'idle' | 'loading' | 'ok' | 'error'>,
  skipped: Record<string, boolean>,
  lockedSources: Record<string, boolean> = {},
): boolean {
  return defs
    .filter((d) => d.requiresApiKey)
    .every((d) => statuses[d.id] === 'ok' || skipped[d.id] || lockedSources[d.id])
}

export function canProceedFromRoots(modules: ModuleState, roots: Record<ModuleKey, string[]>): boolean {
  return (Object.keys(modules) as ModuleKey[])
    .filter((k) => modules[k])
    .every((k) => roots[k].length > 0 && roots[k].every((p) => p.length > 0 && p.startsWith('/')))
}

// ── Step 1: Welcome ───────────────────────────────────────────────────────────

function WelcomeStep({ onNext }: { onNext: () => void }) {
  return (
    <div className="flex flex-col items-center text-center gap-8">
      <div className="flex flex-col items-center gap-4">
        <div className="w-20 h-20 rounded-2xl bg-indigo-600/20 border border-indigo-500/30 flex items-center justify-center">
          <svg className="w-10 h-10 text-indigo-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 3v4M3 5h4M6 17v4m-2-2h4m5-16l2.286 6.857L21 12l-5.714 2.143L13 21l-2.286-6.857L5 12l5.714-2.143L13 3z" />
          </svg>
        </div>
        <div className="flex flex-col gap-2">
          <h1 className="text-4xl font-bold text-white tracking-tight">Purser</h1>
          <p className="text-lg text-gray-400 max-w-sm leading-relaxed">
            A self-hosted metadata manager for the media you care about.
          </p>
        </div>
      </div>

      <div className="w-full rounded-xl border border-gray-800 bg-gray-900/50 p-5 text-left flex flex-col gap-3">
        <p className="text-sm font-semibold text-gray-300 uppercase tracking-wider">What to expect</p>
        <ul className="flex flex-col gap-2">
          {[
            'Choose which media modules you want to use',
            'Configure your metadata sources',
            'Connect your storage locations',
            'Start enriching your library',
          ].map((item) => (
            <li key={item} className="flex items-start gap-2.5 text-sm text-gray-400">
              <svg className="w-4 h-4 text-indigo-400 mt-0.5 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4" />
              </svg>
              {item}
            </li>
          ))}
        </ul>
      </div>

      <button
        onClick={onNext}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        Get Started
      </button>
    </div>
  )
}

// ── Step 2: Module selection ──────────────────────────────────────────────────

interface ModulesStepProps {
  modules:  ModuleState
  locked:   Record<string, boolean>
  onToggle: (key: ModuleKey) => void
  onNext:   () => void
}

function ModulesStep({ modules, locked, onToggle, onNext }: ModulesStepProps) {
  const keys = Object.keys(MODULE_META) as ModuleKey[]
  const anyEnabled = keys.some((k) => modules[k])

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h2 className="text-2xl font-bold text-white">Choose your modules</h2>
        <p className="text-sm text-gray-400">
          Only enabled modules appear in the sidebar. You can change this later in settings.
        </p>
      </div>

      <ul className="flex flex-col gap-3">
        {keys.map((key) => {
          const { label, description, icon } = MODULE_META[key]
          const enabled = modules[key]
          const isLocked = locked[`modules.${key}.enabled`] ?? false
          return (
            <li key={key}>
              <button
                onClick={() => { if (!isLocked) onToggle(key) }}
                disabled={isLocked}
                aria-disabled={isLocked}
                className={[
                  'w-full text-left rounded-xl border p-4 transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950',
                  isLocked
                    ? 'border-gray-700 bg-gray-900/40 opacity-60 cursor-not-allowed'
                    : enabled
                      ? 'border-indigo-500/50 bg-indigo-950/40 hover:bg-indigo-950/60'
                      : 'border-gray-800 bg-gray-900/40 hover:border-gray-700 hover:bg-gray-900/60',
                ].join(' ')}
              >
                <div className="flex items-start justify-between gap-4">
                  <div className="flex items-start gap-3">
                    <span className="text-2xl leading-none mt-0.5" aria-hidden="true">{icon}</span>
                    <div className="flex flex-col gap-0.5">
                      <span className={`font-semibold text-sm ${enabled ? 'text-white' : 'text-gray-300'}`}>
                        {label}
                      </span>
                      <span className="text-xs text-gray-500 leading-relaxed">{description}</span>
                      {isLocked && (
                        <span className="text-xs text-gray-600 mt-0.5">
                          Provided by <span className="font-mono">{configKeyToEnvVar(`modules.${key}.enabled`)}</span> or your config file — cannot be changed here
                        </span>
                      )}
                    </div>
                  </div>
                  <div
                    className={[
                      'relative shrink-0 mt-0.5 w-10 h-6 rounded-full transition-colors duration-200',
                      enabled ? 'bg-indigo-500' : 'bg-gray-700',
                    ].join(' ')}
                    aria-checked={enabled}
                    role="switch"
                  >
                    <span
                      className={[
                        'absolute top-1 left-1 w-4 h-4 rounded-full bg-white shadow transition-transform duration-200',
                        enabled ? 'translate-x-4' : 'translate-x-0',
                      ].join(' ')}
                    />
                  </div>
                </div>
              </button>
            </li>
          )
        })}
      </ul>

      <button
        onClick={onNext}
        disabled={!anyEnabled}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-800 disabled:text-gray-600 disabled:cursor-not-allowed text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        Next
      </button>
    </div>
  )
}

// ── Step 3: Metadata sources ──────────────────────────────────────────────────

type SourceStatus = 'idle' | 'loading' | 'ok' | 'error'

export interface SourceRowState {
  apiKey:       string
  endpointUrl:  string
  status:       SourceStatus
  errorMessage: string
  skipped:      boolean
  locked:       boolean
}

export function makeInitialSourceState(
  defs: SourceDef[],
  configLocked: Record<string, boolean> = {},
): Record<string, SourceRowState> {
  return Object.fromEntries(
    defs.map((d) => {
      const prefix = SOURCE_CONFIG_PREFIX[d.id]
      const apiKeyLocked = d.requiresApiKey && (configLocked[`${prefix}.api_key`] ?? false)
      return [
        d.id,
        {
          apiKey:       apiKeyLocked ? '***' : '',
          endpointUrl:  d.defaultEndpointUrl ?? '',
          status:       'idle' as SourceStatus,
          errorMessage: '',
          skipped:      false,
          locked:       apiKeyLocked,
        },
      ]
    }),
  )
}

interface SourceRowProps {
  def:                SourceDef
  state:              SourceRowState
  onApiKeyChange:     (value: string) => void
  onEndpointUrlChange:(value: string) => void
  onTest:             () => void
  onSkip:             () => void
}

function SourceRow({ def, state, onApiKeyChange, onEndpointUrlChange, onTest, onSkip }: SourceRowProps) {
  const { status, skipped, errorMessage, locked } = state

  return (
    <div className="rounded-xl border border-gray-800 bg-gray-900/40 p-4 flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <span className="font-semibold text-sm text-white">{def.label}</span>
        {status === 'ok' ? (
          <span className="flex items-center gap-1.5 text-xs text-green-400 font-medium">
            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
            </svg>
            Connected
          </span>
        ) : locked ? (
          <span className="text-xs text-gray-500 font-medium">Pre-configured</span>
        ) : skipped ? (
          <span className="text-xs text-gray-500 font-medium">Skipped</span>
        ) : null}
      </div>

      {!def.requiresApiKey ? (
        <p className="text-xs text-gray-500">Pre-configured — no API key required.</p>
      ) : locked ? (
        <div className="flex flex-col gap-1.5">
          <input
            type="text"
            value="***"
            disabled
            className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-gray-500 opacity-50 cursor-not-allowed"
          />
          <p className="text-xs text-gray-600">
            Provided by{' '}
            <span className="font-mono">{configKeyToEnvVar(`${SOURCE_CONFIG_PREFIX[def.id]}.api_key`)}</span>
            {' '}or your config file — cannot be changed here
          </p>
          {errorMessage && (
            <p className="text-xs text-red-400">{errorMessage}</p>
          )}
          <button
            onClick={onTest}
            disabled={status === 'loading' || status === 'ok'}
            className="self-start flex items-center gap-2 px-3 py-1.5 rounded-lg bg-gray-700 hover:bg-gray-600 disabled:bg-gray-700 disabled:text-gray-500 disabled:cursor-not-allowed text-white text-xs font-semibold transition-colors"
          >
            {status === 'loading' && (
              <svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
              </svg>
            )}
            Test connection
          </button>
        </div>
      ) : skipped ? (
        <button
          onClick={onSkip}
          className="text-xs text-indigo-400 hover:text-indigo-300 text-left transition-colors"
        >
          Undo skip
        </button>
      ) : (
        <>
          {def.hasEndpointUrl && (
            <input
              type="url"
              placeholder="Endpoint URL"
              value={state.endpointUrl}
              onChange={(e) => onEndpointUrlChange(e.target.value)}
              disabled={status === 'loading' || status === 'ok'}
              className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50"
            />
          )}
          <input
            type="password"
            placeholder="API Key"
            value={state.apiKey}
            onChange={(e) => onApiKeyChange(e.target.value)}
            disabled={status === 'loading' || status === 'ok'}
            className="w-full rounded-lg border border-gray-700 bg-gray-800 px-3 py-2 text-sm text-white placeholder-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500 disabled:opacity-50"
          />

          {errorMessage && (
            <p className="text-xs text-red-400">{errorMessage}</p>
          )}

          <div className="flex items-center gap-3">
            <button
              onClick={onTest}
              disabled={status === 'loading' || status === 'ok'}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-700 disabled:text-gray-500 disabled:cursor-not-allowed text-white text-xs font-semibold transition-colors"
            >
              {status === 'loading' && (
                <svg className="w-3.5 h-3.5 animate-spin" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                </svg>
              )}
              Test connection
            </button>
            <button
              onClick={onSkip}
              className="text-xs text-gray-500 hover:text-gray-300 transition-colors"
            >
              Skip
            </button>
          </div>
        </>
      )}
    </div>
  )
}

interface MetadataSourcesStepProps {
  modules:      ModuleState
  configLocked: Record<string, boolean>
  onNext:       (states: Record<string, SourceRowState>) => void
  loading:      boolean
}

function MetadataSourcesStep({ modules, configLocked, onNext, loading }: MetadataSourcesStepProps) {
  const defs = sourcesForModules(modules)
  const [sourceStates, setSourceStates] = useState<Record<string, SourceRowState>>(
    () => makeInitialSourceState(defs, configLocked),
  )
  const { mutate: verifySource } = useVerifySource()

  function updateSource(id: string, patch: Partial<SourceRowState>) {
    setSourceStates((prev) => ({ ...prev, [id]: { ...prev[id], ...patch } }))
  }

  function handleTest(def: SourceDef) {
    updateSource(def.id, { status: 'loading', errorMessage: '' })
    verifySource(def.id, {
      onSuccess: (res) => {
        if (res.ok) {
          updateSource(def.id, { status: 'ok', errorMessage: '' })
        } else {
          updateSource(def.id, { status: 'error', errorMessage: res.error ?? 'Verification failed.' })
        }
      },
      onError: (err) => {
        updateSource(def.id, {
          status: 'error',
          errorMessage: err instanceof Error ? err.message : 'Verification failed.',
        })
      },
    })
  }

  function handleSkip(id: string) {
    setSourceStates((prev) => ({
      ...prev,
      [id]: { ...prev[id], skipped: !prev[id].skipped, status: 'idle', errorMessage: '' },
    }))
  }

  const statuses = Object.fromEntries(
    Object.entries(sourceStates).map(([id, s]) => [id, s.status]),
  )
  const skipped = Object.fromEntries(
    Object.entries(sourceStates).map(([id, s]) => [id, s.skipped]),
  )
  const lockedSources = Object.fromEntries(
    Object.entries(sourceStates).map(([id, s]) => [id, s.locked]),
  )
  const canProceed = canProceedFromSources(defs, statuses, skipped, lockedSources)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h2 className="text-2xl font-bold text-white">Metadata sources</h2>
        <p className="text-sm text-gray-400">
          Connect the sources that power your library. Test each key before continuing.
        </p>
      </div>

      <div className="flex flex-col gap-3">
        {defs.map((def) => (
          <SourceRow
            key={def.id}
            def={def}
            state={sourceStates[def.id]}
            onApiKeyChange={(v) => updateSource(def.id, { apiKey: v, status: 'idle', errorMessage: '' })}
            onEndpointUrlChange={(v) => updateSource(def.id, { endpointUrl: v, status: 'idle', errorMessage: '' })}
            onTest={() => handleTest(def)}
            onSkip={() => handleSkip(def.id)}
          />
        ))}
      </div>

      <button
        onClick={() => onNext(sourceStates)}
        disabled={!canProceed || loading}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-800 disabled:text-gray-600 disabled:cursor-not-allowed text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        {loading ? 'Saving…' : 'Next'}
      </button>
    </div>
  )
}

// ── Step 4: Media roots ───────────────────────────────────────────────────────

interface MediaRootsStepProps {
  modules:      ModuleState
  roots:        Record<ModuleKey, string[]>
  locked:       Record<string, boolean>
  onRootChange: (key: ModuleKey, index: number, value: string) => void
  onRootAdd:    (key: ModuleKey) => void
  onRootRemove: (key: ModuleKey, index: number) => void
  onNext:       () => void
}

function MediaRootsStep({ modules, roots, locked, onRootChange, onRootAdd, onRootRemove, onNext }: MediaRootsStepProps) {
  const keys = (Object.keys(MODULE_META) as ModuleKey[]).filter((k) => modules[k])
  const canProceed = canProceedFromRoots(modules, roots)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-1">
        <h2 className="text-2xl font-bold text-white">Media roots</h2>
        <p className="text-sm text-gray-400">
          Set the root directories on disk where each module's content lives.
        </p>
      </div>

      <div className="flex flex-col gap-5">
        {keys.map((key) => {
          const { label, icon } = MODULE_META[key]
          const paths = roots[key]
          const isLocked = locked[`modules.${key}.roots`] ?? false
          return (
            <div key={key} className="flex flex-col gap-2">
              <div className="flex items-center gap-2">
                <span className="flex items-center gap-2 text-sm font-medium text-gray-300">
                  <span aria-hidden="true">{icon}</span>
                  {label}
                </span>
                {isLocked && (
                  <span className="text-xs text-gray-500 font-medium">operator-managed</span>
                )}
              </div>
              <div className="flex flex-col gap-1.5">
                {paths.map((path, i) => {
                  const invalid = !isLocked && path.length > 0 && !path.startsWith('/')
                  return (
                    <div key={i} className="flex flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <input
                          type="text"
                          placeholder={`/media/${key}`}
                          value={path}
                          onChange={(e) => onRootChange(key, i, e.target.value)}
                          disabled={isLocked}
                          className={[
                            'flex-1 rounded-lg border bg-gray-800 px-3 py-2 text-sm placeholder-gray-600 focus:outline-none focus:ring-2 focus:ring-indigo-500',
                            isLocked
                              ? 'border-gray-700 text-gray-500 opacity-50 cursor-not-allowed'
                              : invalid
                                ? 'border-red-500 text-white'
                                : 'border-gray-700 text-white',
                          ].join(' ')}
                        />
                        {!isLocked && paths.length > 1 && (
                          <button
                            onClick={() => onRootRemove(key, i)}
                            aria-label="Remove directory"
                            className="shrink-0 w-8 h-8 flex items-center justify-center rounded-lg text-gray-500 hover:text-red-400 hover:bg-gray-800 transition-colors"
                          >
                            <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
                              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                          </button>
                        )}
                      </div>
                      {invalid && (
                        <p className="text-xs text-red-400 pl-0.5">Path must start with /</p>
                      )}
                    </div>
                  )
                })}
              </div>
              {isLocked ? (
                <p className="text-xs text-gray-600">
                  Provided by{' '}
                  <span className="font-mono">{configKeyToEnvVar(`modules.${key}.roots`)}</span>
                  {' '}or your config file — cannot be changed here
                </p>
              ) : (
                <button
                  onClick={() => onRootAdd(key)}
                  className="self-start flex items-center gap-1.5 text-xs text-indigo-400 hover:text-indigo-300 transition-colors"
                >
                  <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4v16m8-8H4" />
                  </svg>
                  Add directory
                </button>
              )}
            </div>
          )
        })}
      </div>

      <button
        onClick={onNext}
        disabled={!canProceed}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-800 disabled:text-gray-600 disabled:cursor-not-allowed text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        Next
      </button>
    </div>
  )
}

// ── Step 5: Done ──────────────────────────────────────────────────────────────

interface DoneStepProps {
  modules:    ModuleState
  roots:      Record<ModuleKey, string[]>
  locked:     Record<string, boolean>
  onComplete: () => void
  loading:    boolean
}

function DoneStep({ modules, roots, locked, onComplete, loading }: DoneStepProps) {
  const keys = (Object.keys(MODULE_META) as ModuleKey[]).filter((k) => modules[k])

  return (
    <div className="flex flex-col gap-8">
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="w-16 h-16 rounded-2xl bg-green-600/20 border border-green-500/30 flex items-center justify-center">
          <svg className="w-8 h-8 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div className="flex flex-col gap-1">
          <h2 className="text-2xl font-bold text-white">You're all set</h2>
          <p className="text-sm text-gray-400">Here's a summary of your configuration.</p>
        </div>
      </div>

      <div className="rounded-xl border border-gray-800 bg-gray-900/50 divide-y divide-gray-800">
        {keys.map((key) => {
          const { label, icon } = MODULE_META[key]
          const rootsLocked = locked[`modules.${key}.roots`] ?? false
          return (
            <div key={key} className="flex items-start justify-between gap-4 px-4 py-3">
              <div className="flex items-center gap-2 shrink-0">
                <span aria-hidden="true">{icon}</span>
                <span className="text-sm font-medium text-white">{label}</span>
              </div>
              <div className="flex flex-col items-end gap-0.5">
                {rootsLocked ? (
                  <span className="text-xs text-gray-500 font-medium">operator-managed</span>
                ) : (
                  roots[key].map((path, i) => (
                    <span key={i} className="text-sm text-gray-400 font-mono text-right break-all">{path}</span>
                  ))
                )}
              </div>
            </div>
          )
        })}
      </div>

      <button
        onClick={onComplete}
        disabled={loading}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-700 disabled:text-gray-500 disabled:cursor-not-allowed text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        {loading ? 'Setting up…' : 'Go to library'}
      </button>
    </div>
  )
}

// ── Config patch builder ──────────────────────────────────────────────────────

function buildConfigPatch(
  modules: ModuleState,
  roots: Record<ModuleKey, string[]>,
  sourceStates: Record<string, SourceRowState>,
  locked: Record<string, boolean>,
): PatchConfigRequest {
  const modulePatch: NonNullable<PatchConfigRequest['modules']> = {}
  for (const key of Object.keys(modules) as ModuleKey[]) {
    const entry: { enabled?: boolean; roots?: string[] } = {}
    if (!locked[`modules.${key}.enabled`]) entry.enabled = modules[key]
    if (!locked[`modules.${key}.roots`]) {
      const validRoots = roots[key].filter((p) => p.length > 0 && p.startsWith('/'))
      if (validRoots.length > 0) entry.roots = validRoots
    }
    if (Object.keys(entry).length > 0) modulePatch[key] = entry
  }

  const sourcesPatch: NonNullable<PatchConfigRequest['sources']> = {}
  for (const [id, state] of Object.entries(sourceStates)) {
    const srcId = id as SourceID
    const prefix = SOURCE_CONFIG_PREFIX[srcId]
    const patchKey = SOURCE_PATCH_KEY[srcId]
    const entry: { api_key?: string; url?: string } = {}
    if (!locked[`${prefix}.api_key`] && state.apiKey && state.apiKey !== '***') {
      entry.api_key = state.apiKey
    }
    if (srcId === 'stashdb' && !locked[`${prefix}.url`] && state.endpointUrl) {
      entry.url = state.endpointUrl
    }
    if (Object.keys(entry).length > 0) {
      ;(sourcesPatch as Record<string, typeof entry>)[patchKey] = entry
    }
  }

  const body: PatchConfigRequest = {}
  if (Object.keys(modulePatch).length > 0) body.modules = modulePatch
  if (Object.keys(sourcesPatch).length > 0) body.sources = sourcesPatch
  return body
}

// ── Wizard shell ──────────────────────────────────────────────────────────────

export const DEFAULT_ROOTS: Record<ModuleKey, string[]> = {
  movies:    ['/media/movies'],
  tv:        ['/media/tv'],
  music:     ['/media/music'],
  afterdark: ['/media/afterdark'],
  books:     ['/media/books'],
}

export function SetupPage() {
  const [step, setStep]       = useState(1)
  const [modules, setModules] = useState<ModuleState>(DEFAULT_MODULES)
  const [roots, setRoots]     = useState<Record<ModuleKey, string[]>>(DEFAULT_ROOTS)
  const [savedSourceStates, setSavedSourceStates] = useState<Record<string, SourceRowState>>({})
  const [configInitDone, setConfigInitDone] = useState(false)

  const navigate      = useNavigate()
  const queryClient   = useQueryClient()
  const { data: config }                      = useAppConfig()
  const { mutate: patchConfig, isPending: isPatchPending } = usePatchConfig()
  const { mutate: completeSetup, isPending: isCompletePending } = useCompleteSetup()
  const isPending = isPatchPending || isCompletePending

  const locked = config?.locked ?? {}

  useEffect(() => {
    if (!config || configInitDone) return
    setConfigInitDone(true)
    setModules({
      movies:    config.modules.movies.enabled,
      tv:        config.modules.tv.enabled,
      music:     config.modules.music.enabled,
      afterdark: config.modules.afterdark.enabled,
      books:     config.modules.books.enabled,
    })
    setRoots({
      movies:    config.modules.movies.roots.length > 0 ? config.modules.movies.roots : DEFAULT_ROOTS.movies,
      tv:        config.modules.tv.roots.length > 0 ? config.modules.tv.roots : DEFAULT_ROOTS.tv,
      music:     config.modules.music.roots.length > 0 ? config.modules.music.roots : DEFAULT_ROOTS.music,
      afterdark: config.modules.afterdark.roots.length > 0 ? config.modules.afterdark.roots : DEFAULT_ROOTS.afterdark,
      books:     config.modules.books.roots.length > 0 ? config.modules.books.roots : DEFAULT_ROOTS.books,
    })
  }, [config, configInitDone])

  function toggleModule(key: ModuleKey) {
    setModules((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  function updateRoot(key: ModuleKey, index: number, value: string) {
    setRoots((prev) => {
      const updated = [...prev[key]]
      updated[index] = value
      return { ...prev, [key]: updated }
    })
  }

  function addRoot(key: ModuleKey) {
    setRoots((prev) => ({ ...prev, [key]: [...prev[key], ''] }))
  }

  function removeRoot(key: ModuleKey, index: number) {
    setRoots((prev) => ({ ...prev, [key]: prev[key].filter((_, i) => i !== index) }))
  }

  function handleComplete() {
    const body = buildConfigPatch(modules, roots, savedSourceStates, locked)
    const finish = () => completeSetup(undefined, {
      onSuccess: () => {
        queryClient.setQueryData(['setup', 'status'], { complete: true })
        navigate('/', { replace: true })
      },
    })

    if (Object.keys(body).length > 0) {
      patchConfig(body, { onSuccess: finish })
    } else {
      finish()
    }
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gray-950 px-4 py-12">
      <div className="w-full max-w-md flex flex-col gap-10">
        <StepIndicator current={step} total={TOTAL_STEPS} />

        {step === 1 && <WelcomeStep onNext={() => setStep(2)} />}
        {step === 2 && (
          <ModulesStep
            modules={modules}
            locked={locked}
            onToggle={toggleModule}
            onNext={() => setStep(3)}
          />
        )}
        {step === 3 && (
          <MetadataSourcesStep
            modules={modules}
            configLocked={locked}
            onNext={(states) => { setSavedSourceStates(states); setStep(4) }}
            loading={false}
          />
        )}
        {step === 4 && (
          <MediaRootsStep
            modules={modules}
            roots={roots}
            locked={locked}
            onRootChange={updateRoot}
            onRootAdd={addRoot}
            onRootRemove={removeRoot}
            onNext={() => setStep(5)}
          />
        )}
        {step === 5 && (
          <DoneStep
            modules={modules}
            roots={roots}
            locked={locked}
            onComplete={handleComplete}
            loading={isPending}
          />
        )}
      </div>
    </div>
  )
}
