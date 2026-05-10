// Builds Source -> Application edges from the cached lists. The
// Application DTO carries `source` as either "<Kind>/<name>" (the
// usual case) or just "<name>" (when the controller couldn't infer
// a kind — uncommon). We resolve by (clusterId, ns, kind?, name)
// and accept the same-namespace assumption: today the API doesn't
// surface sourceRef.namespace, so cross-namespace refs render as
// orphans. That's a deliberate v1 limitation; future revisions
// should plumb the namespace through `applications.go`.

import type { Application, Source } from './types'

export type EdgeTone = 'ok' | 'warn' | 'err' | 'muted'

export interface DepEdge {
  sourceId: string // Source.id
  appId: string // Application.id
  tone: EdgeTone
}

export interface ClusterDepGraph {
  clusterId: string
  clusterName: string
  // sources / apps lists are sorted: failing first, then alphabetical.
  // Only includes sources that have at least one dependent app and
  // apps that have a resolvable source — the unmatched ones land in
  // orphanApps / unusedSources so the rendering layer can treat
  // them separately (no edges to draw).
  sources: Source[]
  apps: Application[]
  edges: DepEdge[]
  orphanApps: Application[]
  unusedSources: Source[]
}

export function parseSourceRef(s: string | undefined): { kind?: string; name: string } | null {
  if (!s) return null
  const idx = s.indexOf('/')
  if (idx < 0) return { name: s }
  return { kind: s.slice(0, idx), name: s.slice(idx + 1) }
}

export function buildClusterGraphs(apps: Application[], sources: Source[]): ClusterDepGraph[] {
  // Group both lists by clusterId. We seed the map from clusters that
  // have apps OR sources so a cluster that has only one of the two
  // still shows up (with an empty other column).
  const buckets = new Map<string, { name: string; apps: Application[]; sources: Source[] }>()
  const seed = (clusterId: string, displayName: string) => {
    if (!buckets.has(clusterId)) {
      buckets.set(clusterId, { name: displayName, apps: [], sources: [] })
    }
  }
  for (const a of apps) {
    seed(a.clusterId, a.cluster)
    buckets.get(a.clusterId)!.apps.push(a)
  }
  for (const s of sources) {
    seed(s.clusterId, s.cluster)
    buckets.get(s.clusterId)!.sources.push(s)
  }

  const result: ClusterDepGraph[] = []
  for (const [clusterId, { name, apps: cApps, sources: cSources }] of buckets) {
    // Build a lookup that handles both the kinded and non-kinded
    // source-ref formats. We prefer the kinded match when available.
    const byNsKindName = new Map<string, Source>()
    const byNsName = new Map<string, Source[]>()
    for (const s of cSources) {
      byNsKindName.set(`${s.ns}/${s.kind}/${s.name}`, s)
      const k = `${s.ns}/${s.name}`
      if (!byNsName.has(k)) byNsName.set(k, [])
      byNsName.get(k)!.push(s)
    }

    const edges: DepEdge[] = []
    const orphanApps: Application[] = []
    const usedSources = new Set<string>()
    const matchedApps: Application[] = []

    for (const a of cApps) {
      const ref = parseSourceRef(a.source)
      if (!ref) continue // app advertises no source ref (rare)
      let src: Source | undefined
      if (ref.kind) src = byNsKindName.get(`${a.ns}/${ref.kind}/${ref.name}`)
      if (!src) {
        const candidates = byNsName.get(`${a.ns}/${ref.name}`)
        if (candidates && candidates.length === 1) src = candidates[0]
        else if (candidates && candidates.length > 1 && ref.kind) {
          src = candidates.find((c) => c.kind === ref.kind) ?? undefined
        }
      }
      if (!src) {
        orphanApps.push(a)
        continue
      }
      matchedApps.push(a)
      usedSources.add(src.id)
      edges.push({
        sourceId: src.id,
        appId: a.id,
        tone: edgeTone(src, a),
      })
    }

    const usedSourceList = cSources.filter((s) => usedSources.has(s.id))
    const unusedSources = cSources.filter((s) => !usedSources.has(s.id))

    result.push({
      clusterId,
      clusterName: name,
      sources: usedSourceList.sort(severityNameSort),
      apps: matchedApps.sort(severityNameSort),
      edges,
      orphanApps: orphanApps.sort(severityNameSort),
      unusedSources: unusedSources.sort(severityNameSort),
    })
  }

  return result.sort((a, b) => a.clusterName.localeCompare(b.clusterName))
}

function edgeTone(s: Source, a: Application): EdgeTone {
  if (s.status === 'failing' || a.status === 'failing') return 'err'
  if (s.status === 'degraded' || a.status === 'degraded') return 'warn'
  if (a.suspended) return 'muted'
  return 'ok'
}

// severityNameSort puts failing items first, then degraded, then the
// rest alphabetically. Used uniformly across both columns so the
// "most operationally interesting" rows are anchored at the top.
function severityNameSort<T extends { status: string; name: string }>(a: T, b: T): number {
  const rank = (s: string) =>
    s === 'failing' ? 0 : s === 'degraded' ? 1 : s === 'progressing' ? 2 : 3
  const r = rank(a.status) - rank(b.status)
  if (r !== 0) return r
  return a.name.localeCompare(b.name)
}
