import { useImageUpdates, useClusters } from '@/lib/queries'
import type { Cluster } from '@/lib/types'
import { ClusterCard } from '@/components/ClusterCard'
import { Sparkline } from '@/components/Sparkline'
import { Stat } from '@/components/Stat'
import { Ic } from '@/components/Icons'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'

export type FleetLayout = 'cards' | 'matrix' | 'map'

interface Props {
  layout: FleetLayout
  setLayout: (l: FleetLayout) => void
  clusterId: string | null
  pickCluster: (id: string) => void
}

export function FleetPage({ layout, setLayout, clusterId, pickCluster }: Props) {
  const { data, isLoading, error, refetch, isFetching } = useClusters()
  const { data: imgData } = useImageUpdates()
  const clusters = data?.clusters ?? []
  const imageUpdates = imgData?.updates ?? []

  const totalApps = clusters.reduce((a, c) => a + c.apps, 0)
  const totalReady = clusters.reduce((a, c) => a + c.ready, 0)
  const totalFail = clusters.reduce((a, c) => a + c.failing, 0)
  const totalSusp = clusters.reduce((a, c) => a + c.suspended, 0)
  const readyPct = totalApps > 0 ? ((totalReady / totalApps) * 100).toFixed(1) : '—'

  // Image-update stat: "pending" means an ImagePolicy has resolved a newer
  // tag than the live image — that's the actual operator-facing signal.
  const imageReady = imageUpdates.filter((u) => u.status === 'ready' && u.latestTag).length
  const imageFailing = imageUpdates.filter((u) => u.status === 'failing').length

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Fleet
            <span className="meta">
              {clusters.length} cluster{clusters.length === 1 ? '' : 's'} · {totalApps} apps · live
            </span>
            <span className="pulse" title="Live" />
          </h1>
          <div className="page-sub">Reconciliation health across every Flux-managed cluster.</div>
        </div>
        <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <div className="seg" role="tablist">
            <button
              className={layout === 'cards' ? 'active' : ''}
              onClick={() => setLayout('cards')}
            >
              Cards
            </button>
            <button
              className={layout === 'matrix' ? 'active' : ''}
              onClick={() => setLayout('matrix')}
            >
              Matrix
            </button>
            <button className={layout === 'map' ? 'active' : ''} onClick={() => setLayout('map')}>
              Compact
            </button>
          </div>
          <button
            className="btn"
            onClick={() => void refetch()}
            disabled={isFetching}
            title="Refetch cluster status now"
          >
            <Ic.refresh /> {isFetching ? 'Refreshing…' : 'Refresh'}
          </button>
        </div>
      </div>

      <div className="stats">
        <Stat
          label="Apps reconciling"
          value={totalReady}
          of={totalApps}
          delta={
            <>
              <span style={{ color: 'var(--ok)' }}>● {readyPct}%</span> ready
            </>
          }
          tone="ok"
        />
        <Stat
          label="Failing"
          value={totalFail}
          delta={<>{clusters.filter((c) => c.failing > 0).length} clusters</>}
          tone={totalFail > 0 ? 'err' : ''}
        />
        <Stat label="Suspended" value={totalSusp} tone="warn" accent="var(--paused)" />
        <Stat
          label="Image updates"
          value={imageReady}
          of={imageUpdates.length || undefined}
          delta={
            imageUpdates.length === 0 ? (
              <>no ImagePolicies</>
            ) : imageFailing > 0 ? (
              <span style={{ color: 'var(--err)' }}>{imageFailing} failing</span>
            ) : (
              <>resolved</>
            )
          }
          tone={imageFailing > 0 ? 'err' : ''}
        />
      </div>

      {isLoading && clusters.length === 0 && <LoadingState label="Loading clusters…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && clusters.length === 0 && (
        <EmptyState
          title="No clusters registered"
          hint={
            <>
              In CRD mode, create a <span className="mono">Cluster</span> CR pointing at a
              kubeconfig Secret. In file mode, set <span className="mono">--config-file</span> or{' '}
              <span className="mono">--kubeconfig</span>.
            </>
          }
        />
      )}

      {clusters.length > 0 && layout === 'cards' && (
        <div className="cluster-grid">
          {clusters.map((c) => (
            <ClusterCard
              key={c.id}
              c={asCardCluster(c)}
              active={c.id === clusterId}
              onClick={() => pickCluster(c.id)}
            />
          ))}
        </div>
      )}

      {clusters.length > 0 && layout === 'matrix' && (
        <FleetMatrix clusters={clusters} activeId={clusterId} onPick={pickCluster} />
      )}
      {clusters.length > 0 && layout === 'map' && (
        <FleetMap clusters={clusters} activeId={clusterId} onPick={pickCluster} />
      )}
    </>
  )
}

// asCardCluster adapts the API DTO to the shape ClusterCard wants.
function asCardCluster(c: Cluster) {
  return {
    id: c.id,
    name: c.name,
    region: c.region ?? '',
    env: c.env ?? '',
    status: c.status,
    apps: c.apps,
    ready: c.ready,
    failing: c.failing,
    suspended: c.suspended,
    sources: c.sources,
    version: c.version ?? '',
    spark: c.spark ?? [],
  }
}

function FleetMatrix({
  clusters,
  activeId,
  onPick,
}: {
  clusters: Cluster[]
  activeId: string | null
  onPick: (id: string) => void
}) {
  return (
    <div className="panel" style={{ overflow: 'hidden' }}>
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">View</span>Cluster status matrix
        </div>
      </div>
      <div style={{ overflowX: 'auto' }}>
        <table className="tbl">
          <thead>
            <tr>
              <th style={{ minWidth: 220 }}>Cluster</th>
              <th>Env</th>
              <th>Apps</th>
              <th>Ready</th>
              <th>Failing</th>
              <th>Suspended</th>
              <th>Sources</th>
              <th>Flux</th>
              <th>Health</th>
            </tr>
          </thead>
          <tbody>
            {clusters.map((c) => (
              <tr
                key={c.id}
                className={c.id === activeId ? 'selected' : ''}
                onClick={() => onPick(c.id)}
              >
                <td>
                  <span
                    className="row-status"
                    style={{
                      background:
                        c.status === 'failing' || c.status === 'unreachable'
                          ? 'var(--err)'
                          : c.status === 'degraded'
                            ? 'var(--warn)'
                            : 'var(--ok)',
                    }}
                  />
                  <span className="name mono">{c.name}</span>
                  {c.region && (
                    <span className="ns" style={{ marginLeft: 8 }}>
                      {c.region}
                    </span>
                  )}
                </td>
                <td className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
                  {c.env || '—'}
                </td>
                <td className="mono tnum">{c.apps}</td>
                <td
                  className="mono tnum"
                  style={{ color: c.ready > 0 ? 'var(--ok)' : 'var(--ink-3)' }}
                >
                  {c.ready}
                </td>
                <td
                  className="mono tnum"
                  style={{ color: c.failing > 0 ? 'var(--err)' : 'var(--ink-3)' }}
                >
                  {c.failing}
                </td>
                <td
                  className="mono tnum"
                  style={{ color: c.suspended > 0 ? 'var(--paused)' : 'var(--ink-3)' }}
                >
                  {c.suspended}
                </td>
                <td className="mono tnum">{c.sources}</td>
                <td>
                  <span className={`chip ${c.fluxInstalled ? 'ok' : 'warn'}`}>
                    <span className="d" />
                    {c.fluxInstalled ? 'ok' : 'missing'}
                  </span>
                </td>
                <td style={{ width: 140 }}>
                  <div style={{ width: 110, height: 24 }}>
                    <Sparkline
                      data={c.spark ?? []}
                      status={c.failing > 0 ? (c.status === 'failing' ? 'err' : 'warn') : 'ok'}
                      width={110}
                      height={24}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function FleetMap({
  clusters,
  activeId,
  onPick,
}: {
  clusters: Cluster[]
  activeId: string | null
  onPick: (id: string) => void
}) {
  return (
    <div className="panel">
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">View</span>Compact pill list
        </div>
      </div>
      <div className="panel-body">
        <div
          className="fleet-map"
          style={{
            minHeight: 120,
            display: 'flex',
            flexWrap: 'wrap',
            gap: 8,
            padding: 16,
            position: 'relative',
          }}
        >
          {clusters.map((c) => {
            const tone =
              c.status === 'failing' || c.status === 'unreachable'
                ? 'var(--err)'
                : c.status === 'degraded'
                  ? 'var(--warn)'
                  : 'var(--ok)'
            return (
              <button
                key={c.id}
                onClick={() => onPick(c.id)}
                style={{
                  background: 'var(--panel)',
                  border: `1px solid ${c.id === activeId ? 'var(--accent)' : 'var(--line)'}`,
                  borderRadius: 4,
                  padding: '4px 8px',
                  cursor: 'pointer',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 6,
                  height: 'fit-content',
                }}
              >
                <span style={{ width: 6, height: 6, borderRadius: '50%', background: tone }} />
                <span className="mono" style={{ fontSize: 10.5 }}>
                  {c.name}
                </span>
                {c.failing > 0 && (
                  <span className="mono" style={{ fontSize: 10, color: 'var(--err)' }}>
                    {c.failing}!
                  </span>
                )}
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
