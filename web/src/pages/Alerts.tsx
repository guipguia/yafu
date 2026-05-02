import { useAlerts } from '@/lib/queries'
import { StatusChip } from '@/components/StatusChip'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'

export function AlertsPage() {
  const { data, isLoading, error } = useAlerts()
  const alerts = data?.alerts ?? []
  const fanoutErrors = data?.errors ?? []

  const paused = alerts.filter((a) => a.suspended).length

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Alerts &amp; Notifications
            <span className="meta">
              {alerts.length} total{paused > 0 ? ` · ${paused} paused` : ''}
            </span>
          </h1>
          <div className="page-sub">
            notification.toolkit.fluxcd.io Alerts joined with their referenced Provider.
          </div>
        </div>
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

      {isLoading && alerts.length === 0 && <LoadingState label="Loading alerts…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && alerts.length === 0 && (
        <EmptyState
          title="No alerts"
          hint="Configure Alerts + Providers under notification.toolkit.fluxcd.io to route Flux events to Slack, PagerDuty, MS Teams, webhooks, etc."
        />
      )}

      {alerts.length > 0 && (
        <div className="panel">
          <table className="tbl">
            <thead>
              <tr>
                <th />
                <th>Alert</th>
                <th>Cluster</th>
                <th>Provider</th>
                <th>Severity</th>
                <th>Target</th>
                <th>Status</th>
                <th>Age</th>
              </tr>
            </thead>
            <tbody>
              {alerts.map((a) => (
                <tr key={a.id}>
                  <td>
                    <span
                      className="row-status"
                      style={{
                        background:
                          a.status === 'paused' ? 'var(--paused)' :
                          a.severity === 'error' ? 'var(--err)' :
                          'var(--ok)',
                      }}
                    />
                  </td>
                  <td>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span className="name">{a.name}</span>
                      <span className="ns">{a.ns}</span>
                    </div>
                  </td>
                  <td className="mono" style={{ fontSize: 11.5 }}>{a.cluster}</td>
                  <td className="mono" style={{ fontSize: 11.5, color: a.provider === 'missing' ? 'var(--err)' : 'var(--ink-2)' }}>
                    {a.provider}
                  </td>
                  <td>
                    <span className={`chip ${a.severity === 'error' ? 'err' : 'info'}`}>
                      <span className="d" />
                      {a.severity}
                    </span>
                  </td>
                  <td className="mono" style={{ fontSize: 11.5, color: 'var(--ink-2)' }}>{a.target || '—'}</td>
                  <td><StatusChip status={a.status === 'paused' ? 'paused' : 'healthy'} /></td>
                  <td className="ago">{a.age}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  )
}
