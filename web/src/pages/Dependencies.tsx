import { useLayoutEffect, useMemo, useRef, useState } from 'react'
import type { Application, Source } from '@/lib/types'
import { useApplications, useSources } from '@/lib/queries'
import {
  buildClusterGraphs,
  type ClusterDepGraph,
  type DepEdge,
  type EdgeTone,
} from '@/lib/depGraph'
import { Ic } from '@/components/Icons'
import { StatusChip } from '@/components/StatusChip'
import { KindBadge } from '@/components/KindBadge'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'

interface Props {
  onOpenApp: (app: Application) => void
}

type StatusFilter = 'all' | 'failing' | 'degraded'

export function DependenciesPage({ onOpenApp }: Props) {
  const appsQ = useApplications()
  const sourcesQ = useSources()
  const apps = useMemo(() => appsQ.data?.applications ?? [], [appsQ.data?.applications])
  const sources = useMemo(() => sourcesQ.data?.sources ?? [], [sourcesQ.data?.sources])
  const fanout = [...(appsQ.data?.errors ?? []), ...(sourcesQ.data?.errors ?? [])]
  const isLoading = appsQ.isLoading || sourcesQ.isLoading
  const error = appsQ.error || sourcesQ.error

  const [statusFilter, setStatusFilter] = useState<StatusFilter>('all')
  const [search, setSearch] = useState('')
  const [clusterFilter, setClusterFilter] = useState<string>('all')

  const allGraphs = useMemo(() => buildClusterGraphs(apps, sources), [apps, sources])

  // Apply filters per-cluster. We filter the rendered list but
  // computed totals show the pre-filter picture so the user can
  // tell the difference between "no edges" and "edges hidden".
  const graphs = useMemo(() => {
    return allGraphs
      .filter((g) => clusterFilter === 'all' || g.clusterId === clusterFilter)
      .map((g) => filterGraph(g, statusFilter, search))
      .filter((g) => g.edges.length + g.orphanApps.length + g.unusedSources.length > 0)
  }, [allGraphs, clusterFilter, statusFilter, search])

  const totals = useMemo(
    () => ({
      edges: allGraphs.reduce((n, g) => n + g.edges.length, 0),
      failingEdges: allGraphs.reduce(
        (n, g) => n + g.edges.filter((e) => e.tone === 'err').length,
        0,
      ),
      orphans: allGraphs.reduce((n, g) => n + g.orphanApps.length, 0),
      unused: allGraphs.reduce((n, g) => n + g.unusedSources.length, 0),
    }),
    [allGraphs],
  )

  const clusters = useMemo(
    () => allGraphs.map((g) => ({ id: g.clusterId, name: g.clusterName })),
    [allGraphs],
  )

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Dependencies
            <span className="meta">
              {totals.edges} edges
              {totals.failingEdges > 0 ? ` · ${totals.failingEdges} failing` : ''}
              {totals.orphans > 0 ? ` · ${totals.orphans} orphan` : ''}
              {totals.unused > 0 ? ` · ${totals.unused} unused` : ''}
            </span>
          </h1>
          <div className="page-sub">
            Source → application graph. Joins on (cluster, namespace, kind, name). Apps whose
            sourceRef points outside their own namespace surface as orphans (the API doesn't carry
            sourceRef.namespace yet).
          </div>
        </div>
      </div>

      <div className="filter-bar">
        <div className="search" style={{ width: 320 }}>
          <Ic.search />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Filter by source or app name…"
            aria-label="Filter dependencies"
          />
        </div>
        <div className="seg" role="radiogroup" aria-label="Status filter">
          {(['all', 'failing', 'degraded'] as StatusFilter[]).map((s) => (
            <button
              key={s}
              type="button"
              role="radio"
              aria-checked={statusFilter === s}
              className={statusFilter === s ? 'active' : ''}
              onClick={() => setStatusFilter(s)}
            >
              {s === 'all' ? 'All' : s === 'failing' ? 'Failing only' : 'Degraded+'}
            </button>
          ))}
        </div>
        {clusters.length > 1 && (
          <div className="seg" role="radiogroup" aria-label="Cluster filter">
            <button
              type="button"
              role="radio"
              aria-checked={clusterFilter === 'all'}
              className={clusterFilter === 'all' ? 'active' : ''}
              onClick={() => setClusterFilter('all')}
            >
              All clusters
            </button>
            {clusters.map((c) => (
              <button
                key={c.id}
                type="button"
                role="radio"
                aria-checked={clusterFilter === c.id}
                className={clusterFilter === c.id ? 'active' : ''}
                onClick={() => setClusterFilter(c.id)}
              >
                {c.name}
              </button>
            ))}
          </div>
        )}
      </div>

      {fanout.length > 0 && (
        <div
          className="panel"
          style={{ padding: '10px 14px', marginBottom: 12, borderLeft: '2px solid var(--warn)' }}
        >
          <span className="mono" style={{ fontSize: 11, color: 'var(--warn)' }}>
            partial fan-out:
          </span>{' '}
          {fanout.map((e, i) => (
            <span
              key={`${e.cluster}-${i}`}
              className="mono"
              style={{ fontSize: 11.5, color: 'var(--ink-3)' }}
            >
              {i > 0 && ' · '}
              {e.cluster}: {e.error}
            </span>
          ))}
        </div>
      )}

      {isLoading && allGraphs.length === 0 && <LoadingState label="Loading dependency graph…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && allGraphs.length === 0 && (
        <EmptyState
          title="No dependencies"
          hint="Need at least one Kustomization or HelmRelease with a resolvable sourceRef."
        />
      )}
      {!isLoading && !error && allGraphs.length > 0 && graphs.length === 0 && (
        <EmptyState
          title="No matches"
          hint="Adjust the filters to see edges, orphans, or unused sources."
        />
      )}

      {graphs.map((g) => (
        <ClusterGraphBlock key={g.clusterId} graph={g} onOpenApp={onOpenApp} />
      ))}
    </>
  )
}

function filterGraph(
  g: ClusterDepGraph,
  statusFilter: StatusFilter,
  search: string,
): ClusterDepGraph {
  const q = search.trim().toLowerCase()
  const wantStatus = (s: string, suspended?: boolean) => {
    if (statusFilter === 'all') return true
    if (suspended) return false
    if (statusFilter === 'failing') return s === 'failing'
    return s === 'failing' || s === 'degraded'
  }
  const wantSearch = (...fields: string[]) => !q || fields.some((f) => f.toLowerCase().includes(q))

  // Filter edges first: an edge stays if BOTH endpoints survive
  // both filters. Then derive the visible sources / apps from the
  // surviving edges so the columns don't show disconnected nodes.
  const survivingEdges: DepEdge[] = []
  const sourceById = new Map(g.sources.map((s) => [s.id, s]))
  const appById = new Map(g.apps.map((a) => [a.id, a]))

  for (const e of g.edges) {
    const s = sourceById.get(e.sourceId)
    const a = appById.get(e.appId)
    if (!s || !a) continue
    if (!wantStatus(s.status, s.suspended)) continue
    if (!wantStatus(a.status, a.suspended)) continue
    if (!wantSearch(s.name, s.ns, a.name, a.ns)) continue
    survivingEdges.push(e)
  }

  const keptSrcIds = new Set(survivingEdges.map((e) => e.sourceId))
  const keptAppIds = new Set(survivingEdges.map((e) => e.appId))

  return {
    ...g,
    sources: g.sources.filter((s) => keptSrcIds.has(s.id)),
    apps: g.apps.filter((a) => keptAppIds.has(a.id)),
    edges: survivingEdges,
    orphanApps: g.orphanApps.filter(
      (a) => wantStatus(a.status, a.suspended) && wantSearch(a.name, a.ns, a.source ?? ''),
    ),
    unusedSources: g.unusedSources.filter(
      (s) => wantStatus(s.status, s.suspended) && wantSearch(s.name, s.ns),
    ),
  }
}

interface ClusterBlockProps {
  graph: ClusterDepGraph
  onOpenApp: (app: Application) => void
}

function ClusterGraphBlock({ graph, onOpenApp }: ClusterBlockProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const sourceRefs = useRef<Map<string, HTMLElement>>(new Map())
  const appRefs = useRef<Map<string, HTMLElement>>(new Map())
  const [layout, setLayout] = useState<LayoutState | null>(null)
  const [hover, setHover] = useState<{ kind: 'source' | 'app'; id: string } | null>(null)

  // Re-measure after every layout pass and whenever the window
  // resizes. We don't rebuild on hover because the cards' positions
  // are stable — only the SVG stroke colors change.
  useLayoutEffect(() => {
    const measure = () => {
      const container = containerRef.current
      if (!container) return
      const containerRect = container.getBoundingClientRect()
      const points = new Map<string, { x: number; y: number }>()
      const tagPoint = (id: string, el: HTMLElement | undefined, side: 'right' | 'left') => {
        if (!el) return
        const r = el.getBoundingClientRect()
        points.set(id, {
          x: (side === 'right' ? r.right : r.left) - containerRect.left,
          y: r.top + r.height / 2 - containerRect.top,
        })
      }
      for (const s of graph.sources) tagPoint(`src:${s.id}`, sourceRefs.current.get(s.id), 'right')
      for (const a of graph.apps) tagPoint(`app:${a.id}`, appRefs.current.get(a.id), 'left')
      setLayout({ points, width: containerRect.width, height: containerRect.height })
    }
    measure()
    const ro = new ResizeObserver(measure)
    if (containerRef.current) ro.observe(containerRef.current)
    window.addEventListener('resize', measure)
    return () => {
      ro.disconnect()
      window.removeEventListener('resize', measure)
    }
  }, [graph])

  const dimmed = (kind: 'source' | 'app', id: string): boolean => {
    if (!hover) return false
    if (hover.kind === kind && hover.id === id) return false
    if (hover.kind === 'source') {
      return !graph.edges.some(
        (e) => e.sourceId === hover.id && (kind === 'app' ? e.appId : e.sourceId) === id,
      )
    }
    // hover on app
    return !graph.edges.some(
      (e) => e.appId === hover.id && (kind === 'source' ? e.sourceId : e.appId) === id,
    )
  }

  const edgeStyle = (e: DepEdge): { stroke: string; opacity: number; width: number } => {
    const isAdjacent =
      !hover ||
      (hover.kind === 'source' && e.sourceId === hover.id) ||
      (hover.kind === 'app' && e.appId === hover.id)
    const colorFor: Record<EdgeTone, string> = {
      ok: 'var(--line-strong)',
      warn: 'var(--warn)',
      err: 'var(--err)',
      muted: 'var(--ink-4)',
    }
    return {
      stroke: colorFor[e.tone],
      opacity: hover ? (isAdjacent ? 1 : 0.12) : e.tone === 'ok' ? 0.5 : 0.85,
      width: isAdjacent && hover ? 1.8 : e.tone === 'err' ? 1.4 : 1,
    }
  }

  return (
    <div className="panel" style={{ marginBottom: 16, overflow: 'hidden' }}>
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">Cluster</span>
          {graph.clusterName}
        </div>
        <div className="panel-actions">
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            {graph.sources.length} src · {graph.apps.length} app · {graph.edges.length} edge
            {graph.orphanApps.length > 0 ? ` · ${graph.orphanApps.length} orphan` : ''}
            {graph.unusedSources.length > 0 ? ` · ${graph.unusedSources.length} unused` : ''}
          </span>
        </div>
      </div>

      <div
        ref={containerRef}
        style={{
          position: 'relative',
          display: 'grid',
          gridTemplateColumns: '1fr 180px 1fr',
          alignItems: 'start',
          gap: 0,
          padding: '16px 14px',
        }}
      >
        <ColumnHeader label="Sources" align="left" />
        <div />
        <ColumnHeader label="Applications" align="right" />

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, gridColumn: 1 }}>
          {graph.sources.map((s) => (
            <SourceCard
              key={s.id}
              source={s}
              cardRef={(el) => {
                if (el) sourceRefs.current.set(s.id, el)
                else sourceRefs.current.delete(s.id)
              }}
              dimmed={dimmed('source', s.id)}
              onHover={(active) => setHover(active ? { kind: 'source', id: s.id } : null)}
            />
          ))}
        </div>

        <div style={{ gridColumn: 2 }} />

        <div style={{ display: 'flex', flexDirection: 'column', gap: 6, gridColumn: 3 }}>
          {graph.apps.map((a) => (
            <AppCard
              key={a.id}
              app={a}
              cardRef={(el) => {
                if (el) appRefs.current.set(a.id, el)
                else appRefs.current.delete(a.id)
              }}
              dimmed={dimmed('app', a.id)}
              onHover={(active) => setHover(active ? { kind: 'app', id: a.id } : null)}
              onClick={() => onOpenApp(a)}
            />
          ))}
        </div>

        {layout && (
          <svg
            aria-hidden="true"
            style={{
              position: 'absolute',
              inset: 0,
              width: '100%',
              height: '100%',
              pointerEvents: 'none',
            }}
          >
            {graph.edges.map((e, i) => {
              const s = layout.points.get(`src:${e.sourceId}`)
              const a = layout.points.get(`app:${e.appId}`)
              if (!s || !a) return null
              const midX = (s.x + a.x) / 2
              const d = `M ${s.x},${s.y} C ${midX},${s.y} ${midX},${a.y} ${a.x},${a.y}`
              const style = edgeStyle(e)
              return (
                <path
                  key={i}
                  d={d}
                  stroke={style.stroke}
                  strokeWidth={style.width}
                  strokeOpacity={style.opacity}
                  fill="none"
                  strokeDasharray={e.tone === 'muted' ? '3 3' : undefined}
                />
              )
            })}
          </svg>
        )}
      </div>

      {(graph.orphanApps.length > 0 || graph.unusedSources.length > 0) && (
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: '1fr 1fr',
            gap: 0,
            borderTop: '1px solid var(--line)',
          }}
        >
          <OrphanList apps={graph.orphanApps} onOpenApp={onOpenApp} />
          <UnusedList sources={graph.unusedSources} />
        </div>
      )}
    </div>
  )
}

interface LayoutState {
  points: Map<string, { x: number; y: number }>
  width: number
  height: number
}

function ColumnHeader({ label, align }: { label: string; align: 'left' | 'right' }) {
  return (
    <div
      className="mono"
      style={{
        gridColumn: align === 'left' ? 1 : 3,
        fontSize: 10,
        color: 'var(--ink-3)',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        padding: '0 4px 8px',
        textAlign: align,
      }}
    >
      {label}
    </div>
  )
}

interface SourceCardProps {
  source: Source
  cardRef: (el: HTMLDivElement | null) => void
  dimmed: boolean
  onHover: (active: boolean) => void
}

function SourceCard({ source, cardRef, dimmed, onHover }: SourceCardProps) {
  return (
    <div
      ref={cardRef}
      onMouseEnter={() => onHover(true)}
      onMouseLeave={() => onHover(false)}
      onFocus={() => onHover(true)}
      onBlur={() => onHover(false)}
      tabIndex={0}
      style={{
        ...cardBase,
        borderLeft: `2px solid ${statusColor(source.status, source.suspended)}`,
        opacity: dimmed ? 0.35 : 1,
        transition: 'opacity 80ms ease',
      }}
    >
      <div style={cardIcon} aria-hidden="true">
        {source.kind === 'GitRepository' ? <Ic.git /> : <Ic.source />}
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={cardTitle}>{source.name}</div>
        <div style={cardSub}>
          <KindBadge kind={source.kind} /> <span style={{ marginLeft: 6 }}>{source.ns}</span>
        </div>
      </div>
      <StatusChip status={source.status} />
    </div>
  )
}

interface AppCardProps {
  app: Application
  cardRef: (el: HTMLDivElement | null) => void
  dimmed: boolean
  onHover: (active: boolean) => void
  onClick: () => void
}

function AppCard({ app, cardRef, dimmed, onHover, onClick }: AppCardProps) {
  return (
    <div
      ref={cardRef}
      onMouseEnter={() => onHover(true)}
      onMouseLeave={() => onHover(false)}
      onFocus={() => onHover(true)}
      onBlur={() => onHover(false)}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          onClick()
        }
      }}
      tabIndex={0}
      role="button"
      aria-label={`Open ${app.kind} ${app.name}`}
      style={{
        ...cardBase,
        cursor: 'pointer',
        borderLeft: `2px solid ${statusColor(app.status, app.suspended)}`,
        opacity: dimmed ? 0.35 : 1,
        transition: 'opacity 80ms ease',
      }}
    >
      <StatusChip status={app.status} />
      <div style={{ flex: 1, minWidth: 0, textAlign: 'right' }}>
        <div style={cardTitle}>{app.name}</div>
        <div style={{ ...cardSub, justifyContent: 'flex-end' }}>
          <span style={{ marginRight: 6 }}>{app.ns}</span>
          <KindBadge kind={app.kind} />
        </div>
      </div>
      <div style={cardIcon} aria-hidden="true">
        <Ic.app />
      </div>
    </div>
  )
}

function OrphanList({
  apps,
  onOpenApp,
}: {
  apps: Application[]
  onOpenApp: (app: Application) => void
}) {
  if (apps.length === 0) return <div />
  return (
    <div style={{ padding: 14, borderRight: '1px solid var(--line)' }}>
      <div
        className="mono"
        style={{
          fontSize: 10,
          color: 'var(--err)',
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          marginBottom: 8,
        }}
      >
        Orphan apps ({apps.length})
      </div>
      <p
        className="mono"
        style={{ fontSize: 11, color: 'var(--ink-3)', margin: '0 0 8px', lineHeight: 1.5 }}
      >
        sourceRef points to a Source that isn't in the app's namespace.
      </p>
      <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 4 }}>
        {apps.map((a) => (
          <li key={a.id}>
            <button
              type="button"
              onClick={() => onOpenApp(a)}
              style={miniRow}
              title={`Open ${a.name}`}
            >
              <span
                className="row-status"
                style={{ background: statusColor(a.status, a.suspended) }}
              />
              <span className="mono" style={{ fontSize: 11.5 }}>
                {a.name}
              </span>
              <span className="ns" style={{ marginLeft: 6 }}>
                {a.ns}
              </span>
              <span
                className="mono"
                style={{ marginLeft: 'auto', fontSize: 10.5, color: 'var(--err)' }}
              >
                {a.source || 'no source ref'}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}

function UnusedList({ sources }: { sources: Source[] }) {
  if (sources.length === 0) return <div />
  return (
    <div style={{ padding: 14 }}>
      <div
        className="mono"
        style={{
          fontSize: 10,
          color: 'var(--paused)',
          textTransform: 'uppercase',
          letterSpacing: '0.08em',
          marginBottom: 8,
        }}
      >
        Unused sources ({sources.length})
      </div>
      <p
        className="mono"
        style={{ fontSize: 11, color: 'var(--ink-3)', margin: '0 0 8px', lineHeight: 1.5 }}
      >
        no application references these.
      </p>
      <ul style={{ listStyle: 'none', margin: 0, padding: 0, display: 'grid', gap: 4 }}>
        {sources.map((s) => (
          <li key={s.id} style={miniRow}>
            <span
              className="row-status"
              style={{ background: statusColor(s.status, s.suspended) }}
            />
            <span className="mono" style={{ fontSize: 11.5 }}>
              {s.name}
            </span>
            <span className="ns" style={{ marginLeft: 6 }}>
              {s.ns}
            </span>
            <KindBadge kind={s.kind} />
          </li>
        ))}
      </ul>
    </div>
  )
}

function statusColor(status: string, suspended?: boolean): string {
  if (suspended) return 'var(--paused)'
  switch (status) {
    case 'failing':
      return 'var(--err)'
    case 'degraded':
      return 'var(--warn)'
    case 'progressing':
      return 'var(--info)'
    case 'paused':
      return 'var(--paused)'
    default:
      return 'var(--ok)'
  }
}

const cardBase: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 10,
  padding: '8px 10px',
  background: 'var(--panel)',
  border: '1px solid var(--line)',
  borderRadius: 4,
  minWidth: 0,
}

const cardIcon: React.CSSProperties = {
  width: 22,
  height: 22,
  display: 'grid',
  placeItems: 'center',
  color: 'var(--ink-3)',
  flex: '0 0 auto',
}

const cardTitle: React.CSSProperties = {
  fontSize: 12.5,
  fontWeight: 500,
  color: 'var(--ink)',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
}

const cardSub: React.CSSProperties = {
  fontFamily: 'var(--font-mono)',
  fontSize: 10.5,
  color: 'var(--ink-3)',
  display: 'flex',
  alignItems: 'center',
  gap: 0,
  marginTop: 2,
}

const miniRow: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 6,
  padding: '4px 6px',
  width: '100%',
  background: 'transparent',
  border: 'none',
  borderBottom: '1px solid var(--line)',
  textAlign: 'left',
  cursor: 'pointer',
  color: 'var(--ink)',
}
