import { describe, expect, it } from 'vitest'
import { buildClusterGraphs, parseSourceRef } from './depGraph'
import type { Application, Source } from './types'

const app = (over: Partial<Application>): Application => ({
  id: over.id ?? 'a',
  name: 'a',
  kind: 'Kustomization',
  ns: 'default',
  cluster: 'c1',
  clusterId: 'c1',
  status: 'healthy',
  sync: 'Synced',
  source: '',
  revision: '',
  age: '1m',
  suspended: false,
  ...over,
})

const src = (over: Partial<Source>): Source => ({
  id: over.id ?? 's',
  name: 's',
  kind: 'GitRepository',
  ns: 'default',
  cluster: 'c1',
  clusterId: 'c1',
  url: '',
  status: 'healthy',
  age: '1m',
  suspended: false,
  ...over,
})

describe('parseSourceRef', () => {
  it('returns null for empty', () => {
    expect(parseSourceRef(undefined)).toBeNull()
    expect(parseSourceRef('')).toBeNull()
  })
  it('splits Kind/Name', () => {
    expect(parseSourceRef('GitRepository/yafu')).toEqual({
      kind: 'GitRepository',
      name: 'yafu',
    })
  })
  it('returns name-only when no kind prefix', () => {
    expect(parseSourceRef('yafu')).toEqual({ name: 'yafu' })
  })
})

describe('buildClusterGraphs', () => {
  it('joins by (cluster, ns, kind, name)', () => {
    const apps = [app({ id: 'a1', name: 'web', ns: 'default', source: 'GitRepository/main' })]
    const sources = [src({ id: 's1', name: 'main', ns: 'default', kind: 'GitRepository' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out).toHaveLength(1)
    expect(out[0].edges).toEqual([{ sourceId: 's1', appId: 'a1', tone: 'ok' }])
    expect(out[0].sources.map((s) => s.id)).toEqual(['s1'])
    expect(out[0].apps.map((a) => a.id)).toEqual(['a1'])
    expect(out[0].orphanApps).toHaveLength(0)
    expect(out[0].unusedSources).toHaveLength(0)
  })

  it('returns app in orphanApps when source is in a different namespace', () => {
    const apps = [app({ id: 'a1', name: 'web', ns: 'default', source: 'GitRepository/main' })]
    const sources = [src({ id: 's1', name: 'main', ns: 'flux-system' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out[0].edges).toHaveLength(0)
    expect(out[0].orphanApps.map((a) => a.id)).toEqual(['a1'])
    expect(out[0].unusedSources.map((s) => s.id)).toEqual(['s1'])
  })

  it('returns source in unusedSources when no app references it', () => {
    const apps = [app({ id: 'a1', source: 'GitRepository/wired' })]
    const sources = [src({ id: 'wired', name: 'wired' }), src({ id: 'lonely', name: 'lonely' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out[0].unusedSources.map((s) => s.id)).toEqual(['lonely'])
    expect(out[0].edges).toHaveLength(1)
  })

  it('propagates failing status into the edge tone', () => {
    const apps = [app({ id: 'a1', source: 'GitRepository/s1' })]
    const sources = [src({ id: 's1', name: 's1', status: 'failing' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out[0].edges[0].tone).toBe('err')
  })

  it('marks suspended edges as muted', () => {
    const apps = [app({ id: 'a1', source: 'GitRepository/s1', suspended: true })]
    const sources = [src({ id: 's1', name: 's1' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out[0].edges[0].tone).toBe('muted')
  })

  it('falls back to name-only match when sourceRef has no kind prefix', () => {
    const apps = [app({ id: 'a1', source: 'main' })]
    const sources = [src({ id: 's1', name: 'main' })]
    const out = buildClusterGraphs(apps, sources)
    expect(out[0].edges).toEqual([{ sourceId: 's1', appId: 'a1', tone: 'ok' }])
  })

  it('groups by cluster', () => {
    const apps = [
      app({ id: 'a1', clusterId: 'c1', cluster: 'C1', source: 'GitRepository/main' }),
      app({ id: 'a2', clusterId: 'c2', cluster: 'C2', source: 'GitRepository/main' }),
    ]
    const sources = [
      src({ id: 's1', clusterId: 'c1', cluster: 'C1', name: 'main' }),
      src({ id: 's2', clusterId: 'c2', cluster: 'C2', name: 'main' }),
    ]
    const out = buildClusterGraphs(apps, sources)
    expect(out.map((g) => g.clusterId).sort()).toEqual(['c1', 'c2'])
    // Apps and sources in different clusters MUST NOT cross-link.
    for (const g of out) {
      expect(g.edges).toHaveLength(1)
      expect(g.sources[0].clusterId).toBe(g.clusterId)
      expect(g.apps[0].clusterId).toBe(g.clusterId)
    }
  })
})
