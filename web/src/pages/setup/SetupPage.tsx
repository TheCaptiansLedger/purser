import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { StepIndicator } from '../../components/ui/StepIndicator'
import { useCompleteSetup } from '../../api/setup'

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
  onToggle: (key: ModuleKey) => void
  onNext:   () => void
  loading:  boolean
}

function ModulesStep({ modules, onToggle, onNext, loading }: ModulesStepProps) {
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
          return (
            <li key={key}>
              <button
                onClick={() => onToggle(key)}
                className={[
                  'w-full text-left rounded-xl border p-4 transition-all duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950',
                  enabled
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
        disabled={!anyEnabled || loading}
        className="w-full py-3 px-6 rounded-xl bg-indigo-600 hover:bg-indigo-500 disabled:bg-gray-800 disabled:text-gray-600 disabled:cursor-not-allowed text-white font-semibold text-base transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 focus:ring-offset-gray-950"
      >
        {loading ? 'Saving…' : 'Next'}
      </button>
    </div>
  )
}

// ── Wizard shell ──────────────────────────────────────────────────────────────

export function SetupPage() {
  const [step, setStep]       = useState(1)
  const [modules, setModules] = useState<ModuleState>(DEFAULT_MODULES)
  const navigate              = useNavigate()
  const { mutate, isPending } = useCompleteSetup()

  function toggleModule(key: ModuleKey) {
    setModules((prev) => ({ ...prev, [key]: !prev[key] }))
  }

  function handleModulesNext() {
    mutate(undefined, {
      onSuccess: () => navigate('/', { replace: true }),
    })
  }

  return (
    <div className="min-h-screen flex flex-col items-center justify-center bg-gray-950 px-4 py-12">
      <div className="w-full max-w-md flex flex-col gap-10">
        <StepIndicator current={step} total={TOTAL_STEPS} />

        {step === 1 && <WelcomeStep onNext={() => setStep(2)} />}
        {step === 2 && (
          <ModulesStep
            modules={modules}
            onToggle={toggleModule}
            onNext={handleModulesNext}
            loading={isPending}
          />
        )}
      </div>
    </div>
  )
}
