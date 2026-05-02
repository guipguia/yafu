import { useEvents } from '@/lib/queries'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'
import { Ic } from '@/components/Icons'

export function ActivityPage() {
  const { data, isLoading, error } = useEvents()
  const events = data?.events ?? []
  const fanoutErrors = data?.errors ?? []

  // Top changers — by event source component.
  const sourceCounts = events.reduce<Record<string, number>>((acc, e) => {
    const k = e.source || 'unknown'
    acc[k] = (acc[k] ?? 0) + 1
    return acc
  }, {})
  const topChangers = Object.entries(sourceCounts)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 5)

  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Activity
            <span className="meta">{events.length} recent events · live</span>
            <span className="pulse" />
          </h1>
          <div className="page-sub">k8s Events emitted by Flux controllers across every reachable cluster.</div>
        </div>
        <div style={{ display: 'flex', gap: 6 }}>
          <button className="btn" disabled title="v0.2"><Ic.filter /> Filter</button>
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

      {isLoading && events.length === 0 && <LoadingState label="Loading events…" />}
      {error && <ErrorState message={error.message} />}
      {!isLoading && !error && events.length === 0 && (
        <EmptyState
          title="No Flux events"
          hint="Events appear here as soon as Flux controllers emit them on Kustomization / HelmRelease / source resources."
        />
      )}

      {events.length > 0 && (
        <div className="split">
          <div className="panel">
            <div className="panel-head">
              <div className="panel-title"><span className="lab">Stream</span>Live activity</div>
            </div>
            <div className="panel-body">
              <div className="timeline">
                {events.map((e) => (
                  <div key={e.id} className={`tl-item ${e.kind}`}>
                    <div className="tl-meta">
                      <span>{formatTime(e.t)}</span>
                      <span>{e.source || 'flux'}</span>
                      <span style={{ color: 'var(--ink-4)' }}>·</span>
                      <span style={{ color: 'var(--accent-ink)' }}>
                        {e.cluster}/{e.ns}/{e.object}
                      </span>
                    </div>
                    <div className="tl-msg">
                      <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)', marginRight: 6 }}>
                        {e.reason}
                      </span>
                      {e.message}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
          <div className="panel">
            <div className="panel-head">
              <div className="panel-title"><span className="lab">Top</span>Most active sources</div>
            </div>
            <div className="panel-body">
              <div className="mono" style={{ fontSize: 11, color: 'var(--ink-3)' }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', padding: '4px 0' }}>
                  <span>Source component</span>
                  <span>events</span>
                </div>
                {topChangers.map(([n, c]) => (
                  <div
                    key={n}
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      padding: '6px 0',
                      borderTop: '1px solid var(--line)',
                      fontSize: 12,
                    }}
                  >
                    <span style={{ fontFamily: 'var(--font-ui)', color: 'var(--ink)' }}>{n}</span>
                    <span className="tnum">{c}</span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}
    </>
  )
}

function formatTime(rfc3339: string): string {
  if (!rfc3339) return ''
  try {
    const d = new Date(rfc3339)
    return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return rfc3339
  }
}
