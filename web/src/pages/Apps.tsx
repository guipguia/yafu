import { useState } from 'react'
import { useApplications, useClusters } from '@/lib/queries'
import type { Application } from '@/lib/types'
import { StatusChip } from '@/components/StatusChip'
import { KindBadge } from '@/components/KindBadge'
import { FilterChip } from '@/components/FilterChip'
import { Ic } from '@/components/Icons'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'

interface Props {
  onOpen: (app: Application) => void
}

interface FilterState {
  status: string
  kind: string
  cluster: string
}

export function AppsPage({ onOpen }: Props) {
  const { data, isLoading, error } = useApplications()
  const { data: clustersData } = useClusters()
  const [filter, setFilter] = useState<FilterState>({ status: 'all', kind: 'all', cluster: 'all' })
  const [q, setQ] = useState('')

  const apps = data?.applications ?? []
  const clusters = clustersData?.clusters ?? []
  const fanoutErrors = data?.errors ?? []

  const filtered = apps.filter((a) => {
    if (filter.status !== 'all' && a.status !== filter.status) return false
    if (filter.kind !== 'all' && a.kind !== filter.kind) return false
    if (filter.cluster !== 'all' && a.clusterId !== filter.cluster) return false
    if (q && !`${a.name} ${a.ns} ${a.cluster}`.toLowerCase().includes(q.toLowerCase())) return false
    return true
  })

  const fail = apps.filter((a) => a.status === 'failing').length
  const susp = apps.filter((a) => a.suspended).length

  const statusOpts = [
    ['all', 'All'],
    ['failing', 'Failing'],
    ['degraded', 'Degraded'],
    ['progressing', 'Progressing'],
    ['healthy', 'Healthy'],
    ['paused', 'Suspended'],
  ] as const

  const kindOpts = [
    ['all', 'All'],
    ['Kustomization', 'Kustomization'],
    ['HelmRelease', 'HelmRelease'],
  ] as const

  const clusterOpts = [
    ['all', 'All'] as const,
    ...clusters.map((c) => [c.id, c.name] as const),
  ]

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Applications
            <span className="meta">
              {apps.length} total · {fail} failing · {susp} suspended
            </span>
          </h1>
          <div className="page-sub">Kustomizations and HelmReleases across all registered clusters.</div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button className="btn"><Ic.refresh /> Reconcile selected</button>
          <button className="btn primary">+ New</button>
        </div>
      </div>

      <div className="filter-bar">
        <div className="search" style={{ width: 320 }}>
          <Ic.search />
          <input
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Filter by name, namespace, cluster…"
          />
        </div>
        <FilterChip
          label="Status"
          value={filter.status}
          options={statusOpts}
          onChange={(v) => setFilter({ ...filter, status: v })}
        />
        <FilterChip
          label="Kind"
          value={filter.kind}
          options={kindOpts}
          onChange={(v) => setFilter({ ...filter, kind: v })}
        />
        <FilterChip
          label="Cluster"
          value={filter.cluster}
          options={clusterOpts}
          onChange={(v) => setFilter({ ...filter, cluster: v })}
        />
        <span className="mono" style={{ marginLeft: 'auto', fontSize: 11, color: 'var(--ink-3)' }}>
          {filtered.length} of {apps.length}
        </span>
      </div>

      {fanoutErrors.length > 0 && (
        <div
          className="panel"
          style={{ padding: '10px 14px', marginBottom: 12, borderLeft: '2px solid var(--warn)' }}
        >
          <span className="mono" style={{ fontSize: 11, color: 'var(--warn)' }}>
            partial fan-out:
          </span>{' '}
          {fanoutErrors.map((e, i) => (
            <span key={e.cluster} className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
              {i > 0 && ' · '}
              {e.cluster}: {e.error}
            </span>
          ))}
        </div>
      )}

      {isLoading && apps.length === 0 && <LoadingState label="Loading applications…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && apps.length === 0 && (
        <EmptyState
          title="No applications"
          hint={
            clusters.length === 0
              ? 'Register a cluster first.'
              : 'Flux has no Kustomizations or HelmReleases on the registered clusters.'
          }
        />
      )}

      {filtered.length > 0 && (
        <div className="panel" style={{ overflow: 'hidden' }}>
          <table className="tbl">
            <thead>
              <tr>
                <th style={{ width: 36 }} />
                <th>Name</th>
                <th>Kind</th>
                <th>Namespace</th>
                <th>Cluster</th>
                <th>Sync</th>
                <th>Source · Revision</th>
                <th>Last Reconcile</th>
                <th style={{ width: 36 }} />
              </tr>
            </thead>
            <tbody>
              {filtered.map((a) => (
                <tr
                  key={a.id}
                  onClick={() => onOpen(a)}
                  tabIndex={0}
                  role="button"
                  aria-label={`Open ${a.kind} ${a.name} in ${a.cluster}`}
                  onKeyDown={(e) => {
                    // Only fire when the row itself has focus, so Enter on
                    // the inner ⋯ button doesn't also open the drawer.
                    if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
                      e.preventDefault()
                      onOpen(a)
                    }
                  }}
                >
                  <td>
                    <span
                      className="row-status"
                      style={{
                        background:
                          a.status === 'failing' ? 'var(--err)' :
                          a.status === 'degraded' ? 'var(--warn)' :
                          a.status === 'progressing' ? 'var(--info)' :
                          a.status === 'paused' ? 'var(--paused)' :
                          'var(--ok)',
                      }}
                    />
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span className="name">{a.name}</span>
                      {a.suspended && (
                        <span className="chip paused" style={{ height: 16 }}><Ic.pause /></span>
                      )}
                    </div>
                  </td>
                  <td><KindBadge kind={a.kind} /></td>
                  <td className="ns">{a.ns}</td>
                  <td className="mono" style={{ fontSize: 11.5 }}>{a.cluster}</td>
                  <td><StatusChip status={a.sync} /></td>
                  <td>
                    <span className="mono" style={{ fontSize: 11.5 }}>
                      {a.source || '—'}
                      {a.revision && (
                        <>
                          {' '}<span style={{ color: 'var(--ink-4)' }}>·</span>{' '}
                          <span style={{ color: 'var(--accent-ink)' }}>{a.revision}</span>
                        </>
                      )}
                    </span>
                  </td>
                  <td className="ago">{a.age}</td>
                  <td>
                    <button
                      className="icon-btn"
                      aria-label={`Actions for ${a.name}`}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Ic.more />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
