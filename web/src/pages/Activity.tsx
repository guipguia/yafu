import { useState } from 'react'
import { useEvents } from '@/lib/queries'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'
import { Ic } from '@/components/Icons'

type Severity = 'all' | 'ok' | 'warn' | 'err'

export function ActivityPage() {
  const { data, isLoading, error } = useEvents()
  const allEvents = data?.events ?? []
  const fanoutErrors = data?.errors ?? []

  const [severity, setSeverity] = useState<Severity>('all')
  const [search, setSearch] = useState('')

  // Apply severity + free-text filter on the client. Events come pre-tagged
  // with a kind from the backend (ok/warn/err); search hits reason/message/object.
  const events = allEvents.filter((e) => {
    if (severity !== 'all' && e.kind !== severity) return false
    if (!search) return true
    const q = search.toLowerCase()
    return (
      e.reason.toLowerCase().includes(q) ||
      e.message.toLowerCase().includes(q) ||
      e.object.toLowerCase().includes(q) ||
      e.ns.toLowerCase().includes(q)
    )
  })

  // Top changers — by event source component (pre-filter, so the chart
  // doesn't move around as you filter).
  const sourceCounts = allEvents.reduce<Record<string, number>>((acc, e) => {
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
          <div className="page-sub">
            k8s Events emitted by Flux controllers across every reachable cluster.
          </div>
        </div>
        <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
          <div className="diff-mode-seg" role="group" aria-label="Filter by severity">
            {(['all', 'ok', 'warn', 'err'] as Severity[]).map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => setSeverity(s)}
                aria-pressed={severity === s}
              >
                <span className="pip" />
                {s === 'all' ? 'All' : s === 'ok' ? 'OK' : s === 'warn' ? 'Warnings' : 'Errors'}
              </button>
            ))}
          </div>
          <div className="search" role="search" style={{ minWidth: 220 }}>
            <span aria-hidden="true">
              <Ic.search />
            </span>
            <input
              aria-label="Search events"
              placeholder="Search reason, message, object…"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
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
              <div className="panel-title">
                <span className="lab">Stream</span>Live activity
              </div>
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
                      <span
                        className="mono"
                        style={{ fontSize: 10.5, color: 'var(--ink-3)', marginRight: 6 }}
                      >
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
              <div className="panel-title">
                <span className="lab">Top</span>Most active sources
              </div>
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
