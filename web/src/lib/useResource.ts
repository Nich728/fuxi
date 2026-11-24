import { useCallback, useEffect, useRef, useState } from 'react'

export interface Resource<T> {
  data: T | undefined
  error: Error | undefined
  /** True until the request for the current key (or reload) has answered. */
  loading: boolean
  /** Fetch again, keeping the current data on screen until the new data arrives. */
  reload: () => void
  /** Replace the data locally, e.g. after a successful write. */
  set: (data: T) => void
}

interface Settled<T> {
  id: string // which request this answers
  data?: T
  error?: Error
}

/**
 * Loads data for a screen. Re-runs when `key` changes; ignores responses
 * that arrive after a newer request or after the screen has gone. Old data
 * stays on screen while new data loads.
 */
export function useResource<T>(load: () => Promise<T>, key: unknown[] = []): Resource<T> {
  const [tick, setTick] = useState(0)
  const [settled, setSettled] = useState<Settled<T>>({ id: '' })
  const loadRef = useRef(load)
  useEffect(() => {
    loadRef.current = load
  })

  const id = JSON.stringify([tick, ...key])
  useEffect(() => {
    let live = true
    loadRef.current().then(
      (data) => live && setSettled({ id, data }),
      (e: unknown) => live && setSettled((s) => ({ id, data: s.data, error: e instanceof Error ? e : new Error(String(e)) })),
    )
    return () => {
      live = false
    }
  }, [id])

  const reload = useCallback(() => setTick((t) => t + 1), [])
  const set = useCallback((data: T) => setSettled((s) => ({ ...s, data, error: undefined })), [])
  return { data: settled.data, error: settled.error, loading: settled.id !== id, reload, set }
}
