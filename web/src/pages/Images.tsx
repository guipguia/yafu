import { useImageUpdates, useReconcileImage, useResumeImage, useSuspendImage } from '@/lib/queries'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'
import { Ic } from '@/components/Icons'

export function ImagesPage() {
  const { data, isLoading, error } = useImageUpdates()
  const reconcile = useReconcileImage()
  const suspend = useSuspendImage()
  const resume = useResumeImage()

  const updates = data?.updates ?? []
  const fanoutErrors = data?.errors ?? []
  const lastError = reconcile.error || suspend.error || resume.error

  const ready = updates.filter((u) => u.status === 'ready' && u.latestTag).length
  const failing = updates.filter((u) => u.status === 'failing').length
  const suspended = updates.filter((u) => u.suspended).length

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Image Updates
            <span className="meta">
              {updates.length} ImagePolic{updates.length === 1 ? 'y' : 'ies'}
              {failing > 0 ? ` · ${failing} failing` : ''}
              {ready > 0 ? ` · ${ready} resolved` : ''}
              {suspended > 0 ? ` · ${suspended} paused` : ''}
            </span>
          </h1>
          <div className="page-sub">
            ImagePolicies (image-reflector-controller) joined with their referenced
            ImageRepositories.
          </div>
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

      {isLoading && updates.length === 0 && <LoadingState label="Loading image policies…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && updates.length === 0 && (
        <EmptyState
          title="No ImagePolicies"
          hint="Either Flux's image automation is not installed on any registered cluster, or no ImagePolicies have been created yet."
        />
      )}

      {updates.length > 0 && (
        <div className="panel">
          <table className="tbl">
            <thead>
              <tr>
                <th />
                <th>Policy</th>
                <th>Cluster</th>
                <th>Image</th>
                <th>Latest tag</th>
                <th>Policy rule</th>
                <th>Last scan</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {updates.map((u) => {
                const busy =
                  (reconcile.isPending && reconcile.variables?.id === u.id) ||
                  (suspend.isPending && suspend.variables?.id === u.id) ||
                  (resume.isPending && resume.variables?.id === u.id)
                return (
                  <tr key={u.id}>
                    <td>
                      <span
                        className="row-status"
                        style={{
                          background:
                            u.status === 'failing'
                              ? 'var(--err)'
                              : u.status === 'progressing'
                                ? 'var(--info)'
                                : u.status === 'paused'
                                  ? 'var(--paused)'
                                  : 'var(--ok)',
                        }}
                      />
                    </td>
                    <td>
                      <span className="name">{u.name}</span>
                      <span className="ns" style={{ marginLeft: 8 }}>
                        {u.ns}
                      </span>
                      {u.suspended && (
                        <span className="chip paused" style={{ marginLeft: 8 }}>
                          <Ic.pause /> Suspended
                        </span>
                      )}
                      {u.status === 'failing' && u.message && (
                        <div
                          className="mono"
                          style={{ fontSize: 10.5, color: 'var(--err)', marginTop: 2 }}
                        >
                          {u.message}
                        </div>
                      )}
                    </td>
                    <td className="mono" style={{ fontSize: 11.5 }}>
                      {u.cluster}
                    </td>
                    <td
                      className="mono"
                      style={{
                        fontSize: 11.5,
                        color: u.image ? 'var(--ink-2)' : 'var(--err)',
                      }}
                    >
                      {u.image || 'missing ImageRepository'}
                    </td>
                    <td
                      className="mono"
                      style={{ fontSize: 11.5, color: 'var(--accent-ink)', fontWeight: 600 }}
                    >
                      {u.latestTag ? `↗ ${u.latestTag}` : '—'}
                    </td>
                    <td className="mono" style={{ fontSize: 11 }}>
                      {u.policy || '—'}
                    </td>
                    <td className="ago">{u.age}</td>
                    <td>
                      <div style={{ display: 'flex', gap: 4, justifyContent: 'flex-end' }}>
                        <button
                          className="icon-btn"
                          aria-label={`Reconcile ${u.name}`}
                          title="Reconcile"
                          disabled={busy}
                          onClick={() => reconcile.mutate(u)}
                        >
                          <Ic.refresh />
                        </button>
                        {u.suspended ? (
                          <button
                            className="icon-btn"
                            aria-label={`Resume ${u.name}`}
                            title="Resume"
                            disabled={busy}
                            onClick={() => resume.mutate(u)}
                          >
                            <Ic.play />
                          </button>
                        ) : (
                          <button
                            className="icon-btn"
                            aria-label={`Suspend ${u.name}`}
                            title="Suspend"
                            disabled={busy}
                            onClick={() => suspend.mutate(u)}
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
