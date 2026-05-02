import { useSources, useReconcileSource, useResumeSource, useSuspendSource } from '@/lib/queries'
import { KindBadge } from '@/components/KindBadge'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'
import { Ic } from '@/components/Icons'

export function SourcesPage() {
  const { data, isLoading, error } = useSources()
  const reconcile = useReconcileSource()
  const suspend = useSuspendSource()
  const resume = useResumeSource()

  const sources = data?.sources ?? []
  const fanoutErrors = data?.errors ?? []
  const lastError = reconcile.error || suspend.error || resume.error

  const failing = sources.filter((s) => s.status === 'failing').length
  const byKind = sources.reduce<Record<string, number>>((acc, s) => {
    acc[s.kind] = (acc[s.kind] ?? 0) + 1
    return acc
  }, {})
  const healthy = sources.filter((s) => s.status === 'healthy').length

  const reconcileAll = () => {
    for (const s of sources) reconcile.mutate(s)
  }

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Sources
            <span className="meta">
              {sources.length} total{failing > 0 ? ` · ${failing} failing` : ''}
            </span>
          </h1>
          <div className="page-sub">
            Git repositories, Helm repositories, OCI registries, and Buckets that Flux pulls from.
          </div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button
            className="btn"
            onClick={reconcileAll}
            disabled={sources.length === 0 || reconcile.isPending}
            title="Trigger reconcile on every source"
          >
            <Ic.refresh /> {reconcile.isPending ? 'Reconciling…' : 'Reconcile all'}
          </button>
        </div>
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

      {sources.length > 0 && (
        <div className="stats" style={{ gridTemplateColumns: 'repeat(3, 1fr)' }}>
          <div className="panel" style={{ padding: 14, borderLeft: '2px solid var(--ok)' }}>
            <div
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 9.5,
                color: 'var(--ink-3)',
                textTransform: 'uppercase',
                letterSpacing: '0.1em',
              }}
            >
              Healthy
            </div>
            <div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 6 }}>
              {healthy}{' '}
              <span style={{ fontSize: 12, color: 'var(--ink-3)' }}>/ {sources.length}</span>
            </div>
            <div className="mono" style={{ display: 'flex', gap: 14, marginTop: 8, fontSize: 11 }}>
              {Object.entries(byKind).map(([k, n]) => (
                <span key={k}>
                  {kindShort(k)}: {n}
                </span>
              ))}
            </div>
          </div>
          <div className="panel" style={{ padding: 14, borderLeft: '2px solid var(--err)' }}>
            <div
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 9.5,
                color: 'var(--ink-3)',
                textTransform: 'uppercase',
                letterSpacing: '0.1em',
              }}
            >
              Failing fetches
            </div>
            <div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 6 }}>
              {failing}
            </div>
            {failing > 0 && (
              <div style={{ marginTop: 8, fontSize: 11.5, color: 'var(--ink-3)' }}>
                {sources
                  .filter((s) => s.status === 'failing')
                  .slice(0, 3)
                  .map((s) => s.name)
                  .join(' · ')}
              </div>
            )}
          </div>
          <div className="panel" style={{ padding: 14, borderLeft: '2px solid var(--accent)' }}>
            <div
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 9.5,
                color: 'var(--ink-3)',
                textTransform: 'uppercase',
                letterSpacing: '0.1em',
              }}
            >
              Clusters
            </div>
            <div className="mono" style={{ fontSize: 22, fontWeight: 600, marginTop: 6 }}>
              {new Set(sources.map((s) => s.clusterId)).size}
            </div>
          </div>
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

      {isLoading && sources.length === 0 && <LoadingState label="Loading sources…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && sources.length === 0 && (
        <EmptyState
          title="No sources"
          hint="Flux has no GitRepository / HelmRepository / OCIRepository / Bucket on the registered clusters."
        />
      )}

      {sources.length > 0 && (
        <div className="panel">
          <table className="tbl">
            <thead>
              <tr>
                <th />
                <th>Source</th>
                <th>Kind</th>
                <th>Cluster</th>
                <th>URL</th>
                <th>Ref</th>
                <th>Revision</th>
                <th>Interval</th>
                <th>Last Fetch</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {sources.map((s) => {
                const busy =
                  (reconcile.isPending && reconcile.variables?.id === s.id) ||
                  (suspend.isPending && suspend.variables?.id === s.id) ||
                  (resume.isPending && resume.variables?.id === s.id)
                return (
                  <tr key={s.id}>
                    <td>
                      <span
                        className="row-status"
                        style={{
                          background:
                            s.status === 'failing'
                              ? 'var(--err)'
                              : s.status === 'degraded'
                                ? 'var(--warn)'
                                : s.status === 'progressing'
                                  ? 'var(--info)'
                                  : s.status === 'paused'
                                    ? 'var(--paused)'
                                    : 'var(--ok)',
                        }}
                      />
                    </td>
                    <td>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        {s.kind === 'GitRepository' ? (
                          <Ic.git />
                        ) : s.kind === 'HelmRepository' ? (
                          <Ic.helm />
                        ) : (
                          <Ic.oci />
                        )}
                        <span className="name">{s.name}</span>
                        {s.suspended && (
                          <span className="chip paused" style={{ marginLeft: 6 }}>
                            <Ic.pause /> Suspended
                          </span>
                        )}
                      </div>
                      {s.status === 'failing' && s.message && (
                        <div
                          className="mono"
                          style={{ fontSize: 10.5, color: 'var(--err)', marginTop: 2 }}
                        >
                          {s.message}
                        </div>
                      )}
                    </td>
                    <td>
                      <KindBadge kind={s.kind} />
                    </td>
                    <td className="mono" style={{ fontSize: 11.5 }}>
                      {s.cluster}
                    </td>
                    <td
                      className="mono"
                      style={{
                        fontSize: 11,
                        color: 'var(--ink-3)',
                        maxWidth: 280,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                      }}
                    >
                      {s.url}
                    </td>
                    <td className="mono" style={{ fontSize: 11.5 }}>
                      {s.ref || '—'}
                    </td>
                    <td className="mono" style={{ fontSize: 11, color: 'var(--accent-ink)' }}>
                      {s.revision || '—'}
                    </td>
                    <td className="mono" style={{ fontSize: 11.5 }}>
                      {s.interval || '—'}
                    </td>
                    <td className="ago">{s.age}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}>
                        <button
                          className="icon-btn"
                          aria-label={`Reconcile ${s.name}`}
                          title="Reconcile"
                          disabled={busy}
                          onClick={() => reconcile.mutate(s)}
                        >
                          <Ic.refresh />
                        </button>
                        {s.suspended ? (
                          <button
                            className="icon-btn"
                            aria-label={`Resume ${s.name}`}
                            title="Resume"
                            disabled={busy}
                            onClick={() => resume.mutate(s)}
                          >
                            <Ic.play />
                          </button>
                        ) : (
                          <button
                            className="icon-btn"
                            aria-label={`Suspend ${s.name}`}
                            title="Suspend"
                            disabled={busy}
                            onClick={() => suspend.mutate(s)}
                          >
                            <Ic.pause />
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}

function kindShort(k: string): string {
  switch (k) {
    case 'GitRepository':
      return 'git'
    case 'HelmRepository':
      return 'helm'
    case 'OCIRepository':
      return 'oci'
    case 'Bucket':
      return 'bucket'
    default:
      return k.toLowerCase()
  }
}
