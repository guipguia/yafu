import { useClusters } from '@/lib/queries'
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
  const { data, isLoading, error } = useClusters()
  const clusters = data?.clusters ?? []

  const totalApps = clusters.reduce((a, c) => a + c.apps, 0)
  const totalReady = clusters.reduce((a, c) => a + c.ready, 0)
  const totalFail = clusters.reduce((a, c) => a + c.failing, 0)
  const totalSusp = clusters.reduce((a, c) => a + c.suspended, 0)
  const readyPct = totalApps > 0 ? ((totalReady / totalApps) * 100).toFixed(1) : '—'

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
              Map
            </button>
          </div>
          <button className="btn">
            <Ic.filter /> Filter
          </button>
          <button className="btn">
            <Ic.refresh /> Refresh
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
        <Stat label="Image updates pending" value={'—'} delta={<>v0.2</>} />
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
  const buckets = ['shop', 'identity', 'platform', 'observability', 'ml', 'data', 'web', 'ingress']
  return (
    <div className="panel" style={{ overflow: 'hidden' }}>
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">View</span>Cluster × Namespace matrix
        </div>
        <div className="panel-actions">
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            namespace breakdown lands in v0.2
          </span>
        </div>
      </div>
      <div style={{ overflowX: 'auto' }}>
        <table className="tbl">
          <thead>
            <tr>
              <th style={{ minWidth: 200 }}>Cluster</th>
              {buckets.map((b) => (
                <th key={b} style={{ minWidth: 90 }}>
                  {b}
                </th>
              ))}
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
                {buckets.map((b) => (
                  <td key={b}>
                    <span className="mono" style={{ color: 'var(--ink-4)' }}>
                      —
                    </span>
                  </td>
                ))}
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
          <span className="lab">View</span>Geo distribution
        </div>
        <div className="panel-actions">
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            geo coordinates from Cluster CR · v0.2
          </span>
        </div>
      </div>
      <div className="panel-body">
        <div
          className="fleet-map"
          style={{
            height: 320,
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
