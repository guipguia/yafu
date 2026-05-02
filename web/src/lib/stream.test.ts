import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import { useInvalidationStream, type WatchEvent } from './stream'

// FakeEventSource captures the URL it's opened with and exposes
// fire(eventName, data) so tests can drive the stream from outside.
// Vitest installs it on globalThis.EventSource for the duration of
// the test.
class FakeEventSource {
  static instances: FakeEventSource[] = []
  url: string
  withCredentials?: boolean
  listeners = new Map<string, Set<(e: MessageEvent) => void>>()
  closed = false

  constructor(url: string, init?: { withCredentials?: boolean }) {
    this.url = url
    this.withCredentials = init?.withCredentials
    FakeEventSource.instances.push(this)
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set())
    this.listeners.get(type)!.add(fn)
  }
  removeEventListener(type: string, fn: (e: MessageEvent) => void) {
    this.listeners.get(type)?.delete(fn)
  }
  close() { this.closed = true }
  fire(type: string, data: unknown) {
    const listeners = this.listeners.get(type)
    if (!listeners) return
    const ev = new MessageEvent(type, { data: JSON.stringify(data) })
    for (const fn of listeners) fn(ev)
  }
}

describe('useInvalidationStream', () => {
  let qc: QueryClient
  let originalES: typeof EventSource
  let invalidateSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    vi.useFakeTimers()
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    invalidateSpy = vi.fn()
    qc.invalidateQueries = invalidateSpy
    originalES = globalThis.EventSource
    // @ts-expect-error — runtime monkey-patch for the fake.
    globalThis.EventSource = FakeEventSource
    FakeEventSource.instances = []
  })
  afterEach(() => {
    vi.useRealTimers()
    globalThis.EventSource = originalES
  })

  function withClient(children: React.ReactNode) {
    return React.createElement(QueryClientProvider, { client: qc }, children)
  }

  it('opens an EventSource against /api/v1/stream', () => {
    renderHook(() => useInvalidationStream(), { wrapper: ({ children }) => withClient(children) })
    expect(FakeEventSource.instances).toHaveLength(1)
    expect(FakeEventSource.instances[0].url).toBe('/api/v1/stream')
  })

  it('debounces invalidations within the 300ms window', () => {
    renderHook(() => useInvalidationStream(), { wrapper: ({ children }) => withClient(children) })
    const es = FakeEventSource.instances[0]

    const ev: WatchEvent = { cluster: 'a', kind: 'Kustomization', verb: 'MODIFIED', name: 'foo' }
    es.fire('invalidate', ev)
    es.fire('invalidate', ev)
    es.fire('invalidate', ev)

    // Before debounce window — no invalidation yet.
    expect(invalidateSpy).not.toHaveBeenCalled()

    vi.advanceTimersByTime(310)

    // Each unique queryKey prefix in the Kustomization mapping
    // (applications, app-tree, app-events, app-history, app-manifest,
    // app-diff, app-logs, clusters) gets exactly one invalidation,
    // not three.
    expect(invalidateSpy).toHaveBeenCalledTimes(8)
  })

  it('maps source events to the sources query key', () => {
    renderHook(() => useInvalidationStream(), { wrapper: ({ children }) => withClient(children) })
    const es = FakeEventSource.instances[0]
    es.fire('invalidate', { cluster: 'a', kind: 'GitRepository', verb: 'MODIFIED', name: 'r' })

    vi.advanceTimersByTime(310)

    expect(invalidateSpy).toHaveBeenCalledTimes(1)
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['sources'] })
  })

  it('ignores unknown kinds without erroring', () => {
    renderHook(() => useInvalidationStream(), { wrapper: ({ children }) => withClient(children) })
    const es = FakeEventSource.instances[0]
    es.fire('invalidate', { cluster: 'a', kind: 'NotARealKind', verb: 'MODIFIED', name: 'x' })

    vi.advanceTimersByTime(310)

    expect(invalidateSpy).not.toHaveBeenCalled()
  })

  it('closes the EventSource on unmount', () => {
    const { unmount } = renderHook(() => useInvalidationStream(), {
      wrapper: ({ children }) => withClient(children),
    })
    const es = FakeEventSource.instances[0]
    expect(es.closed).toBe(false)
    unmount()
    expect(es.closed).toBe(true)
  })
})
