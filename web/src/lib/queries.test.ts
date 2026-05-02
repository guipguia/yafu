import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import React from 'react'
import {
  useReconcileApp,
  useReconcileImage,
  useReconcileSource,
  useRollbackApp,
} from './queries'
import type { Application, ImageUpdate, Source } from './types'

// Mutation hooks all wrap fetchJSON; we mock global fetch to assert
// the URL + method + body the hook produces. Each hook covers a
// different mutation surface (apps / images / sources / rollback)
// so a regression in any URL builder shows up as a failing test.

describe('mutation hooks', () => {
  let qc: QueryClient
  let fetchSpy: ReturnType<typeof vi.fn>

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    fetchSpy = vi.fn().mockResolvedValue({
      ok: true,
      status: 202,
      json: async () => ({ status: 'accepted' }),
    } as unknown as Response)
    globalThis.fetch = fetchSpy as unknown as typeof fetch
  })
  afterEach(() => {
    vi.restoreAllMocks()
  })

  function wrapper({ children }: { children: React.ReactNode }) {
    return React.createElement(QueryClientProvider, { client: qc }, children)
  }

  const sampleApp: Application = {
    id: 'alpha/shop/Kustomization/checkout',
    name: 'checkout',
    kind: 'Kustomization',
    ns: 'shop',
    cluster: 'Alpha',
    clusterId: 'alpha',
    status: 'healthy',
    sync: 'Synced',
    source: 'flux/podinfo',
    revision: 'abc123',
    age: '5m',
    suspended: false,
  }

  it('useReconcileApp posts to /api/v1/applications/<cluster>/<ns>/<kind>/<name>/reconcile', async () => {
    const { result } = renderHook(() => useReconcileApp(), { wrapper })
    result.current.mutate(sampleApp)

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())
    const [url, init] = fetchSpy.mock.calls[0]
    expect(url).toBe('/api/v1/applications/alpha/shop/Kustomization/checkout/reconcile')
    expect((init as RequestInit).method).toBe('POST')
  })

  it('useReconcileImage posts to /api/v1/image-updates/.../reconcile', async () => {
    const u: ImageUpdate = {
      id: 'alpha/flux-system/ImagePolicy/podinfo',
      name: 'podinfo',
      cluster: 'Alpha',
      clusterId: 'alpha',
      ns: 'flux-system',
      image: 'ghcr.io/stefanprodan/podinfo',
      latestTag: '6.6.2',
      policy: 'semver:>=6.0.0',
      status: 'ready',
      age: '10m',
      suspended: false,
    }
    const { result } = renderHook(() => useReconcileImage(), { wrapper })
    result.current.mutate(u)

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())
    expect(fetchSpy.mock.calls[0][0]).toBe(
      '/api/v1/image-updates/alpha/flux-system/ImagePolicy/podinfo/reconcile',
    )
  })

  it('useReconcileSource posts to /api/v1/sources/.../reconcile', async () => {
    const s: Source = {
      id: 'alpha/flux/GitRepository/podinfo',
      name: 'podinfo',
      kind: 'GitRepository',
      ns: 'flux',
      cluster: 'Alpha',
      clusterId: 'alpha',
      url: 'https://github.com/stefanprodan/podinfo',
      status: 'healthy',
      age: '1h',
      suspended: false,
    }
    const { result } = renderHook(() => useReconcileSource(), { wrapper })
    result.current.mutate(s)

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())
    expect(fetchSpy.mock.calls[0][0]).toBe(
      '/api/v1/sources/alpha/flux/GitRepository/podinfo/reconcile',
    )
  })

  it('useRollbackApp posts revision in body and hits /rollback', async () => {
    const { result } = renderHook(() => useRollbackApp(), { wrapper })
    result.current.mutate({ app: sampleApp, revision: '6.5.0' })

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())
    const [url, init] = fetchSpy.mock.calls[0]
    expect(url).toBe('/api/v1/applications/alpha/shop/Kustomization/checkout/rollback')
    expect((init as RequestInit).method).toBe('POST')
    const body = JSON.parse((init as RequestInit).body as string)
    expect(body).toEqual({ revision: '6.5.0' })
  })

  it('URL-encodes path components', async () => {
    const weirdApp: Application = {
      ...sampleApp,
      ns: 'team/foo bar',
      name: 'app with space',
    }
    const { result } = renderHook(() => useReconcileApp(), { wrapper })
    result.current.mutate(weirdApp)

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled())
    expect(fetchSpy.mock.calls[0][0]).toBe(
      '/api/v1/applications/alpha/team%2Ffoo%20bar/Kustomization/app%20with%20space/reconcile',
    )
  })
})
