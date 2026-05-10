import { useMemo, useState } from 'react'
import {
  useApplications,
  useClusters,
  useReconcileAllApps,
  useReconcileApp,
  useResumeApp,
  useSuspendApp,
} from '@/lib/queries'
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
  const reconcile = useReconcileApp()
  const suspend = useSuspendApp()
  const resume = useResumeApp()
  const reconcileMany = useReconcileAllApps()
  const [filter, setFilter] = useState<FilterState>({ status: 'all', kind: 'all', cluster: 'all' })
  const [q, setQ] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  // Memoize `apps` against the TanStack Query field directly so the
  // `?? []` fallback doesn't produce a fresh reference each render —
  // otherwise downstream `useMemo` deps churn for free.
  const apps = useMemo(() => data?.applications ?? [], [data?.applications])
  const clusters = clustersData?.clusters ?? []
  const fanoutErrors = data?.errors ?? []
  const lastError = reconcile.error || suspend.error || resume.error || reconcileMany.error

  const filtered = useMemo(
    () =>
      apps.filter((a) => {
        if (filter.status !== 'all' && a.status !== filter.status) return false
        if (filter.kind !== 'all' && a.kind !== filter.kind) return false
        if (filter.cluster !== 'all' && a.clusterId !== filter.cluster) return false
        if (q && !`${a.name} ${a.ns} ${a.cluster}`.toLowerCase().includes(q.toLowerCase())) {
          return false
        }
        return true
      }),
    [apps, filter, q],
  )

  // Selection refers to ids in the *filtered* view — when filters narrow the
  // list, items hidden from the user stop counting as selected.
  const visibleSelected = useMemo(
    () => filtered.filter((a) => selected.has(a.id)),
    [filtered, selected],
  )
  const allVisibleSelected = filtered.length > 0 && visibleSelected.length === filtered.length
  const someVisibleSelected = visibleSelected.length > 0 && !allVisibleSelected

  const toggleOne = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })

  const toggleAll = () =>
    setSelected((prev) => {
      if (allVisibleSelected) {
        // Clear only the currently-visible rows so a narrower filter
        // doesn't wipe selections in clusters the user can't see.
        const next = new Set(prev)
        for (const a of filtered) next.delete(a.id)
        return next
      }
      const next = new Set(prev)
      for (const a of filtered) next.add(a.id)
      return next
    })

  const onReconcileSelected = () => {
    if (visibleSelected.length === 0) return
    reconcileMany.mutate(visibleSelected, {
      onSuccess: () => setSelected(new Set()),
    })
  }

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

  const clusterOpts = [['all', 'All'] as const, ...clusters.map((c) => [c.id, c.name] as const)]

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
          <div className="page-sub">
            Kustomizations and HelmReleases across all registered clusters.
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button
            className="btn primary"
            onClick={onReconcileSelected}
            disabled={visibleSelected.length === 0 || reconcileMany.isPending}
            title={
              visibleSelected.length === 0
                ? 'Select one or more rows to reconcile'
                : `Reconcile ${visibleSelected.length} selected`
            }
          >
            <Ic.refresh />
            {reconcileMany.isPending
              ? 'Reconciling…'
              : visibleSelected.length > 0
                ? `Reconcile ${visibleSelected.length}`
                : 'Reconcile selected'}
          </button>
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

      {lastError && (
        <div
          className="panel"
          style={{ padding: '8px 14px', marginBottom: 12, borderLeft: '2px solid var(--err)' }}
        >
          <span className="mono" style={{ fontSize: 11, color: 'var(--err)' }}>
            action failed:
          </span>{' '}
          <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-2)' }}>
            {lastError.message}
          </span>
        </div>
      )}

      {fanoutErrors.length > 0 && (
        <div
          className="panel"
          style={{ padding: '10px 14px', marginBottom: 12, borderLeft: '2px solid var(--warn)' }}
        >
          <span className="mono" style={{ fontSize: 11, color: 'var(--warn)' }}>
            partial fan-out:
          </span>{' '}
          {fanoutErrors.map((e, i) => (
            <span
              key={e.cluster}
              className="mono"
              style={{ fontSize: 11.5, color: 'var(--ink-3)' }}
            >
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
                <th style={{ width: 32 }}>
                  <input
                    type="checkbox"
                    aria-label={allVisibleSelected ? 'Deselect all rows' : 'Select all rows'}
                    checked={allVisibleSelected}
                    ref={(el) => {
                      if (el) el.indeterminate = someVisibleSelected
                    }}
                    onChange={toggleAll}
                  />
                </th>
                <th style={{ width: 36 }} />
                <th>Name</th>
                <th>Kind</th>
                <th>Namespace</th>
                <th>Cluster</th>
                <th>Sync</th>
                <th>Source · Revision</th>
                <th>Last Reconcile</th>
                <th style={{ width: 72 }} />
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
                  className={selected.has(a.id) ? 'selected' : undefined}
                  onKeyDown={(e) => {
                    // Only fire when the row itself has focus, so Enter on
                    // the inner ⋯ button doesn't also open the drawer.
                    if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
                      e.preventDefault()
                      onOpen(a)
                    }
                  }}
                >
                  <td onClick={(e) => e.stopPropagation()}>
                    <input
                      type="checkbox"
                      aria-label={`Select ${a.name}`}
                      checked={selected.has(a.id)}
                      onChange={() => toggleOne(a.id)}
                    />
                  </td>
                  <td>
                    <span
                      className="row-status"
                      style={{
                        background:
                          a.status === 'failing'
                            ? 'var(--err)'
                            : a.status === 'degraded'
                              ? 'var(--warn)'
                              : a.status === 'progressing'
                                ? 'var(--info)'
                                : a.status === 'paused'
                                  ? 'var(--paused)'
                                  : 'var(--ok)',
                      }}
                    />
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span className="name">{a.name}</span>
                      {a.suspended && (
                        <span className="chip paused" style={{ height: 16 }}>
                          <Ic.pause />
                        </span>
                      )}
                    </div>
                  </td>
                  <td>
                    <KindBadge kind={a.kind} />
                  </td>
                  <td className="ns">{a.ns}</td>
                  <td className="mono" style={{ fontSize: 11.5 }}>
                    {a.cluster}
                  </td>
                  <td>
                    <StatusChip status={a.sync} />
                  </td>
                  <td>
                    <span className="mono" style={{ fontSize: 11.5 }}>
                      {a.source || '—'}
                      {a.revision && (
                        <>
                          {' '}
                          <span style={{ color: 'var(--ink-4)' }}>·</span>{' '}
                          <span style={{ color: 'var(--accent-ink)' }}>{a.revision}</span>
                        </>
                      )}
                    </span>
                  </td>
                  <td className="ago">{a.age}</td>
                  <td>
                    <div
                      style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}
                      onClick={(e) => e.stopPropagation()}
                      onKeyDown={(e) => e.stopPropagation()}
                    >
                      <button
                        className="icon-btn"
                        aria-label={`Reconcile ${a.name}`}
                        title="Reconcile"
                        disabled={
                          (reconcile.isPending && reconcile.variables?.id === a.id) ||
                          (suspend.isPending && suspend.variables?.id === a.id) ||
                          (resume.isPending && resume.variables?.id === a.id)
                        }
                        onClick={() => reconcile.mutate(a)}
                      >
                        <Ic.refresh />
                      </button>
                      {a.suspended ? (
                        <button
                          className="icon-btn"
                          aria-label={`Resume ${a.name}`}
                          title="Resume"
                          disabled={
                            (reconcile.isPending && reconcile.variables?.id === a.id) ||
                            (suspend.isPending && suspend.variables?.id === a.id) ||
                            (resume.isPending && resume.variables?.id === a.id)
                          }
                          onClick={() => resume.mutate(a)}
                        >
                          <Ic.play />
                        </button>
                      ) : (
                        <button
                          className="icon-btn"
                          aria-label={`Suspend ${a.name}`}
                          title="Suspend"
                          disabled={
                            (reconcile.isPending && reconcile.variables?.id === a.id) ||
                            (suspend.isPending && suspend.variables?.id === a.id) ||
                            (resume.isPending && resume.variables?.id === a.id)
                          }
                          onClick={() => suspend.mutate(a)}
                        >
                          <Ic.pause />
                        </button>
                      )}
                    </div>
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
