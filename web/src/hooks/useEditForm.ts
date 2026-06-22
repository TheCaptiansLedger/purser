import { useState, useRef, useCallback } from 'react'

interface UseEditFormOptions<T extends Record<string, unknown>> {
  initial: T
  lockedFields?: string[]
  onSubmit: (values: T, lockedFields: string[]) => Promise<void>
  onSuccess?: () => void
}

export function useEditForm<T extends Record<string, unknown>>({
  initial,
  lockedFields: initialLocked = [],
  onSubmit,
  onSuccess,
}: UseEditFormOptions<T>) {
  const baseValues = useRef(initial)
  const baseLocked = useRef(new Set(initialLocked))

  const onSubmitRef = useRef(onSubmit)
  onSubmitRef.current = onSubmit
  const onSuccessRef = useRef(onSuccess)
  onSuccessRef.current = onSuccess

  const [values, setValues] = useState<T>(initial)
  const [locked, setLocked] = useState<Set<string>>(new Set(initialLocked))
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<Error | null>(null)

  const setField = useCallback(<K extends keyof T>(key: K, value: T[K]) => {
    setValues(prev => ({ ...prev, [key]: value }))
  }, [])

  const toggleLock = useCallback((field: string) => {
    setLocked(prev => {
      const next = new Set(prev)
      if (next.has(field)) {
        next.delete(field)
      } else {
        next.add(field)
      }
      return next
    })
  }, [])

  const isDirty =
    !setsEqual(locked, baseLocked.current) ||
    (Object.keys(baseValues.current) as Array<keyof T>).some(k => values[k] !== baseValues.current[k])

  const submit = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    try {
      await onSubmitRef.current(values, Array.from(locked))
      onSuccessRef.current?.()
    } catch (e) {
      setError(e instanceof Error ? e : new Error(String(e)))
    } finally {
      setSubmitting(false)
    }
  }, [values, locked])

  return { values, setField, lockedFields: locked, toggleLock, isDirty, submit, submitting, error }
}

export function setsEqual(a: Set<string>, b: Set<string>): boolean {
  if (a.size !== b.size) return false
  for (const v of a) if (!b.has(v)) return false
  return true
}
