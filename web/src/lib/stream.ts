import { useEffect } from 'react'
import { useQueryClient, type QueryClient } from '@tanstack/react-query'

// One Server-Sent Event sent by /api/v1/stream when a watched
// FluxCD resource changes on any registered cluster. The frontend
// uses these events to invalidate cached query keys eagerly so the
// UI feels live without spamming the API every few seconds.
export type WatchEvent = {
  cluster: string
  kind: string
  verb: string
  namespace?: string
  name: string
}

// Kind → list of TanStack query-key prefixes to invalidate. Detail
// queries (per-app history/tree/etc) are keyed off these prefixes
// already and will be invalidated automatically.
const INVALIDATIONS: Record<string, string[][]> = {
  Kustomization: [
    ['applications'],
    ['app-tree'],
    ['app-events'],
    ['app-history'],
    ['app-manifest'],
    ['app-diff'],
    ['app-logs'],
    ['clusters'],
  ],
  HelmRelease: [
    ['applications'],
    ['app-tree'],
    ['app-events'],
    ['app-history'],
    ['app-manifest'],
    ['app-diff'],
    ['app-logs'],
    ['clusters'],
  ],
  GitRepository: [['sources']],
  HelmRepository: [['sources']],
  OCIRepository: [['sources']],
  Bucket: [['sources']],
  Alert: [['alerts']],
  Provider: [['alerts']],
  Receiver: [['alerts']],
  ImagePolicy: [['image-updates']],
  ImageRepository: [['image-updates']],
  ImageUpdateAutomation: [['image-updates']],
}

// Coalesce bursts of invalidations: a single Flux reconcile fires
// many MODIFIED events in quick succession (status conditions
// flipping); without debounce we'd refetch the same list 10 times
// per second.
const DEBOUNCE_MS = 300

function scheduleInvalidate(qc: QueryClient, key: string[], pending: Map<string, number>) {
  const id = key.join('|')
  const existing = pending.get(id)
  if (existing) window.clearTimeout(existing)
  const t = window.setTimeout(() => {
    pending.delete(id)
    qc.invalidateQueries({ queryKey: key })
  }, DEBOUNCE_MS)
  pending.set(id, t)
}

// useInvalidationStream subscribes to /api/v1/stream and invalidates
// the query keys mapped from each event's `kind`. The native
// EventSource handles reconnect with exponential backoff for us;
// we just open one and tear it down on unmount.
export function useInvalidationStream() {
  const qc = useQueryClient()
  useEffect(() => {
    let es: EventSource | null = null
    const pending = new Map<string, number>()

    try {
      es = new EventSource('/api/v1/stream', { withCredentials: true })
    } catch {
      return
    }

    const onInvalidate = (e: MessageEvent) => {
      let ev: WatchEvent
      try {
        ev = JSON.parse(e.data) as WatchEvent
      } catch {
        return
      }
      const keys = INVALIDATIONS[ev.kind]
      if (!keys) return
      for (const k of keys) scheduleInvalidate(qc, k, pending)
    }

    es.addEventListener('invalidate', onInvalidate as EventListener)

    return () => {
      es?.removeEventListener('invalidate', onInvalidate as EventListener)
      es?.close()
      for (const t of pending.values()) window.clearTimeout(t)
      pending.clear()
    }
  }, [qc])
}
