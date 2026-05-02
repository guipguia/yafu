import { useState, type CSSProperties, type ReactNode } from 'react'
import type { Application } from '@/lib/types'
import {
  useAppDiff,
  useAppEvents,
  useAppHistory,
  useAppLogs,
  useAppManifest,
  useAppTree,
  useReconcileApp,
  useResumeApp,
  useSuspendApp,
} from '@/lib/queries'
import { StatusChip } from '@/components/StatusChip'
import { EmptyState, ErrorState, LoadingState } from '@/components/States'
import { Ic } from '@/components/Icons'

type Tab = 'overview' | 'tree' | 'diff' | 'events' | 'logs' | 'history' | 'yaml'

const TABS: { id: Tab; label: string }[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'tree', label: 'Resource tree' },
  { id: 'diff', label: 'Diff' },
  { id: 'events', label: 'Events' },
  { id: 'logs', label: 'Logs' },
  { id: 'history', label: 'History' },
  { id: 'yaml', label: 'Manifests' },
]

interface Props {
  app: Application
  onClose: () => void
}

export function AppDetailDrawer({ app, onClose }: Props) {
  const [tab, setTab] = useState<Tab>('overview')
  const reconcile = useReconcileApp()
  const suspend = useSuspendApp()
  const resume = useResumeApp()

  const busy = reconcile.isPending || suspend.isPending || resume.isPending
  const lastError = reconcile.error || suspend.error || resume.error

  const dotColor =
    app.status === 'failing' ? 'var(--err)' :
    app.status === 'degraded' ? 'var(--warn)' :
    app.status === 'paused' ? 'var(--paused)' :
    app.status === 'progressing' ? 'var(--info)' :
    'var(--ok)'

  return (
    <>
      <div className="drawer-scrim" onClick={onClose} />
      <div className="drawer">
        <div className="drawer-head">
          <div className="titles">
            <div className="kind-badge">{app.kind}</div>
            <h2>
              <span style={{ width: 8, height: 8, borderRadius: '50%', background: dotColor }} />
              {app.name}
              {app.suspended && (
                <span className="chip paused"><Ic.pause /> Suspended</span>
              )}
            </h2>
            <div className="ns">
              <span style={{ color: 'var(--ink-2)' }}>{app.cluster}</span>
              <span style={{ color: 'var(--ink-4)', margin: '0 6px' }}>/</span>
              {app.ns}
              {app.source && (
                <>
                  <span style={{ color: 'var(--ink-4)', margin: '0 6px' }}>·</span>
                  {app.source}
                  {app.revision && (
                    <>
                      @<span style={{ color: 'var(--accent-ink)' }}>{app.revision}</span>
                    </>
                  )}
                </>
              )}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            <button className="btn" onClick={() => reconcile.mutate(app)} disabled={busy} title="Trigger reconcile">
              <Ic.refresh /> {reconcile.isPending ? 'Reconciling…' : 'Reconcile'}
            </button>
            {app.suspended ? (
              <button className="btn" onClick={() => resume.mutate(app)} disabled={busy} title="Resume reconciliation">
                <Ic.play /> {resume.isPending ? 'Resuming…' : 'Resume'}
              </button>
            ) : (
              <button className="btn" onClick={() => suspend.mutate(app)} disabled={busy} title="Suspend reconciliation">
                <Ic.pause /> {suspend.isPending ? 'Suspending…' : 'Suspend'}
              </button>
            )}
            <button className="icon-btn" onClick={onClose}><Ic.x /></button>
          </div>
        </div>

        {lastError && (
          <div
            className="panel"
            style={{
              margin: '0 18px 0',
              padding: '8px 12px',
              borderLeft: '2px solid var(--err)',
              borderRadius: 0,
            }}
          >
            <span className="mono" style={{ fontSize: 11, color: 'var(--err)' }}>
              action failed:
            </span>{' '}
            <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-2)' }}>
              {lastError.message}
            </span>
          </div>
        )}

        <div className="tabs">
          {TABS.map((t) => (
            <div
              key={t.id}
              className={`tab ${tab === t.id ? 'active' : ''}`}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </div>
          ))}
        </div>

        <div className="drawer-body">
          {tab === 'overview' && <OverviewTab app={app} />}
          {tab === 'tree' && <TreeTab app={app} />}
          {tab === 'diff' && <DiffTab app={app} />}
          {tab === 'events' && <EventsTab app={app} />}
          {tab === 'logs' && <LogsTab app={app} />}
          {tab === 'history' && <HistoryTab app={app} />}
          {tab === 'yaml' && <ManifestTab app={app} />}
        </div>
      </div>
    </>
  )
}

function OverviewTab({ app }: { app: Application }) {
  return (
    <div style={{ padding: 18, display: 'grid', gap: 14 }}>
      <div className="split">
        <div className="panel">
          <div className="panel-head">
            <div className="panel-title"><span className="lab">Status</span>Reconciliation</div>
            <div className="panel-actions">
              <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
                last reconcile {app.age}
              </span>
            </div>
          </div>
          <div
            className="panel-body"
            style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 10, fontSize: 12 }}
          >
            <KV k="Sync" v={<StatusChip status={app.sync} />} />
            <KV k="Health" v={<StatusChip status={app.status} />} />
            <KV k="Cluster" v={<span className="mono">{app.cluster}</span>} />
            <KV k="Namespace" v={<span className="mono">{app.ns}</span>} />
            <KV k="Source" v={<span className="mono">{app.source || '—'}</span>} />
            <KV k="Revision" v={<span className="mono">{app.revision || '—'}</span>} />
            {app.replicas && <KV k="Replicas" v={<span className="mono tnum">{app.replicas}</span>} />}
            <KV k="Last applied" v={<span className="mono">{app.age}</span>} />
          </div>
        </div>

        <div className="panel">
          <div className="panel-head">
            <div className="panel-title"><span className="lab">Status</span>Message</div>
          </div>
          <div className="panel-body">
            {app.message ? (
              <p
                className="mono"
                style={{ fontSize: 11.5, lineHeight: 1.55, color: 'var(--ink-2)', margin: 0 }}
              >
                {app.message}
              </p>
            ) : (
              <p
                className="mono"
                style={{ fontSize: 11.5, color: 'var(--ink-3)', margin: 0 }}
              >
                no status message reported
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function LogsTab({ app }: { app: Application }) {
  const [pod, setPod] = useState<string | undefined>(undefined)
  const [container, setContainer] = useState<string | undefined>(undefined)
  const { data, isLoading, error } = useAppLogs(app, pod, container, 200)

  const pods = data?.pods ?? []
  const logsText = data?.logs ?? ''
  const note = data?.note ?? ''
  const truncated = !!data?.truncated
  const selectedPod = pods.find((p) => `${p.ns}/${p.name}` === data?.selected)
  const containers = selectedPod?.containers ?? []

  return (
    <div style={{ padding: 18 }}>
      <div className="panel" style={{ background: '#0a0c10', borderColor: '#1f2330' }}>
        <div
          className="panel-head"
          style={{
            background: 'transparent',
            borderColor: '#1f2330',
            flexWrap: 'wrap',
            gap: 8,
          }}
        >
          <div className="panel-title" style={{ color: 'oklch(80% 0 0)' }}>
            <span className="lab" style={{ color: 'oklch(60% 0 0)' }}>Logs</span>
            <select
              value={pod ?? data?.selected ?? ''}
              onChange={(e) => {
                setPod(e.target.value || undefined)
                setContainer(undefined)
              }}
              style={selectStyle}
            >
              {pods.length === 0 && <option value="">no pods discovered</option>}
              {pods.map((p) => (
                <option key={`${p.ns}/${p.name}`} value={`${p.ns}/${p.name}`}>
                  {p.ns}/{p.name} · {p.phase}
                  {p.restarts > 0 ? ` · ${p.restarts} restarts` : ''}
                </option>
              ))}
            </select>
            {containers.length > 1 && (
              <select
                value={container ?? data?.container ?? ''}
                onChange={(e) => setContainer(e.target.value || undefined)}
                style={{ ...selectStyle, marginLeft: 6 }}
              >
                {containers.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            )}
          </div>
          <div className="panel-actions">
            {truncated && (
              <span
                className="mono"
                style={{ fontSize: 10.5, color: 'oklch(78% 0.16 75)' }}
              >
                truncated · last 256 KiB
              </span>
            )}
          </div>
        </div>
        <div
          style={{
            padding: 14,
            fontFamily: 'var(--font-mono)',
            fontSize: 11.5,
            lineHeight: 1.65,
            color: 'oklch(85% 0 0)',
            minHeight: 360,
            maxHeight: 540,
            overflow: 'auto',
            whiteSpace: 'pre',
          }}
        >
          {isLoading && !logsText && (
            <span style={{ color: 'oklch(60% 0 0)' }}>Loading…</span>
          )}
          {error && (
            <span style={{ color: 'oklch(70% 0.18 25)' }}>error: {error.message}</span>
          )}
          {!isLoading && !error && pods.length === 0 && (
            <span style={{ color: 'oklch(60% 0 0)' }}>
              {note || 'No pods matched this application in its inventory namespaces.'}
            </span>
          )}
          {logsText}
          {!logsText && pods.length > 0 && !isLoading && (
            <span style={{ color: 'oklch(60% 0 0)' }}>
              # selected pod produced no log lines yet
            </span>
          )}
        </div>
        {note && (
          <div
            className="mono"
            style={{
              padding: '8px 14px',
              fontSize: 11,
              color: 'oklch(60% 0 0)',
              borderTop: '1px solid #1f2330',
            }}
          >
            note: {note}
          </div>
        )}
      </div>
    </div>
  )
}

const selectStyle: CSSProperties = {
  background: '#1a1d28',
  color: 'oklch(85% 0 0)',
  border: '1px solid #2a2e3a',
  borderRadius: 4,
  padding: '3px 8px',
  fontSize: 11.5,
  fontFamily: 'var(--font-mono)',
}

function DiffTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppDiff(app)
  const resources = data?.resources ?? []
  const drifted = resources.filter((r) => r.status === 'drift').length

  return (
    <div style={{ padding: 18 }}>
      <div className="panel">
        <div className="panel-head">
          <div className="panel-title">
            <span className="lab">Drift</span>
            Field-ownership check{' '}
            <span style={{ color: 'var(--ink-3)', fontWeight: 400 }}>
              ({drifted > 0 ? `${drifted} drifted` : 'none drifted'})
            </span>
          </div>
        </div>
        {isLoading && resources.length === 0 && <LoadingState label="Checking drift…" />}
        {error && <ErrorState message={error.message} />}
        {!isLoading && !error && resources.length === 0 && (
          <EmptyState
            title="No drift"
            hint={
              data?.note ??
              'Either the inventory is empty or every field is owned by Flux controllers.'
            }
          />
        )}
        {resources.length > 0 && (
          <table className="tbl">
            <thead>
              <tr>
                <th />
                <th>Resource</th>
                <th>Status</th>
                <th>Managers</th>
              </tr>
            </thead>
            <tbody>
              {resources.map((r, i) => (
                <tr key={i}>
                  <td>
                    <span
                      className="row-status"
                      style={{
                        background:
                          r.status === 'drift' ? 'var(--err)' :
                          r.status === 'notfound' ? 'var(--paused)' :
                          r.status === 'unknown' ? 'var(--warn)' :
                          'var(--ok)',
                      }}
                    />
                  </td>
                  <td>
                    <span className="kind">{r.kind}</span>
                    <span className="nm" style={{ marginLeft: 6 }}>
                      {r.ns ? <span style={{ color: 'var(--ink-3)' }}>{r.ns}/</span> : null}
                      {r.name}
                    </span>
                  </td>
                  <td>
                    <StatusChip
                      status={r.status === 'drift' ? 'failing' : r.status === 'notfound' ? 'paused' : 'healthy'}
                      label={r.status}
                    />
                  </td>
                  <td>
                    {(r.managers ?? []).length === 0 ? (
                      <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>—</span>
                    ) : (
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6 }}>
                        {r.managers!.map((m, j) => (
                          <span
                            key={j}
                            className="mono"
                            title={`${m.operation}${m.time ? ' · ' + m.time : ''}`}
                            style={{
                              fontSize: 10.5,
                              padding: '1px 6px',
                              borderRadius: 2,
                              border: '1px solid ' + (m.foreign ? 'var(--err)' : 'var(--line-2)'),
                              color: m.foreign ? 'var(--err)' : 'var(--ink-2)',
                              background: m.foreign ? 'var(--err-soft)' : 'var(--bg-2)',
                            }}
                          >
                            {m.manager}
                          </span>
                        ))}
                      </div>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {data?.note && resources.length > 0 && (
          <div
            className="mono"
            style={{
              padding: '8px 14px',
              fontSize: 11,
              color: 'var(--ink-3)',
              borderTop: '1px solid var(--line)',
            }}
          >
            note: {data.note}
          </div>
        )}
      </div>
    </div>
  )
}

function ManifestTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppManifest(app)
  const yamlText = data?.yaml ?? ''

  return (
    <div style={{ padding: 18 }}>
      <div className="panel">
        <div className="panel-head">
          <div className="panel-title">
            <span className="lab">YAML</span>
            {app.kind} · live state
          </div>
          <div className="panel-actions">
            <button
              className="icon-btn"
              title="Copy"
              disabled={!yamlText}
              onClick={() => {
                if (yamlText) {
                  void navigator.clipboard.writeText(yamlText)
                }
              }}
            >
              <Ic.copy />
            </button>
          </div>
        </div>
        {isLoading && !yamlText && <LoadingState label="Loading manifest…" />}
        {error && <ErrorState message={error.message} />}
        {!isLoading && !error && !yamlText && (
          <EmptyState title="No manifest" hint="The resource exists but produced no YAML." />
        )}
        {yamlText && (
          <pre
            style={{
              margin: 0,
              padding: 14,
              fontFamily: 'var(--font-mono)',
              fontSize: 11.5,
              lineHeight: 1.7,
              color: 'var(--ink-2)',
              background: 'var(--bg-2)',
              maxHeight: 540,
              overflow: 'auto',
              whiteSpace: 'pre',
              borderTop: '1px solid var(--line)',
            }}
          >
            {yamlText}
          </pre>
        )}
      </div>
    </div>
  )
}

function TreeTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppTree(app)
  const nodes = data?.nodes ?? []

  return (
    <div style={{ padding: 18 }}>
      <div className="panel">
        <div className="panel-head">
          <div className="panel-title">
            <span className="lab">Tree</span>
            Inventory <span style={{ color: 'var(--ink-3)', fontWeight: 400 }}>({nodes.length})</span>
          </div>
          <div className="panel-actions">
            <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
              owner-ref nesting in v0.3
            </span>
          </div>
        </div>
        {isLoading && nodes.length === 0 && <LoadingState label="Loading inventory…" />}
        {error && <ErrorState message={error.message} />}
        {!isLoading && !error && nodes.length === 0 && (
          <EmptyState
            title="No inventory"
            hint={data?.note ?? "Flux hasn't recorded an inventory for this resource yet."}
          />
        )}
        {nodes.length > 0 && (
          <div className="tree" style={{ padding: '6px 0 8px' }}>
            {nodes.map((n, i) => (
              <div key={i} className="node">
                <span className="twist" />
                <span
                  style={{
                    width: 6,
                    height: 6,
                    borderRadius: '50%',
                    background:
                      n.status === 'failing' ? 'var(--err)' :
                      n.status === 'progressing' ? 'var(--info)' :
                      n.status === 'notfound' ? 'var(--paused)' :
                      n.status === 'unknown' ? 'var(--warn)' :
                      'var(--ok)',
                    flex: '0 0 6px',
                  }}
                />
                <span className="kind">{n.kind}</span>
                <span className="nm">
                  {n.ns ? <span style={{ color: 'var(--ink-3)' }}>{n.ns}/</span> : null}
                  {n.name}
                </span>
                {n.message && (
                  <span
                    className="mono"
                    style={{
                      marginLeft: 'auto',
                      fontSize: 10.5,
                      color:
                        n.status === 'failing' ? 'var(--err)' :
                        n.status === 'notfound' ? 'var(--paused)' :
                        'var(--ink-3)',
                    }}
                  >
                    {n.message}
                  </span>
                )}
              </div>
            ))}
          </div>
        )}
        {data?.note && nodes.length > 0 && (
          <div
            className="mono"
            style={{
              padding: '8px 14px',
              fontSize: 11,
              color: 'var(--ink-3)',
              borderTop: '1px solid var(--line)',
            }}
          >
            note: {data.note}
          </div>
        )}
      </div>
    </div>
  )
}

function HistoryTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppHistory(app)
  const entries = data?.entries ?? []

  return (
    <div style={{ padding: 18 }}>
      <div className="panel">
        <div className="panel-head">
          <div className="panel-title">
            <span className="lab">History</span>Recent revisions
          </div>
          <div className="panel-actions">
            <button className="btn" disabled title="v0.3"><Ic.refresh /> Rollback to selected</button>
          </div>
        </div>
        {isLoading && entries.length === 0 && <LoadingState label="Loading history…" />}
        {error && <ErrorState message={error.message} />}
        {!isLoading && !error && entries.length === 0 && (
          <EmptyState
            title="No history"
            hint={app.kind === 'HelmRelease'
              ? 'No HelmRelease snapshots recorded yet.'
              : 'Kustomization only retains its current revision.'}
          />
        )}
        {entries.length > 0 && (
          <table className="tbl">
            <thead>
              <tr>
                <th />
                <th>Revision</th>
                <th>Action</th>
                <th>App version</th>
                <th>Status</th>
                <th>When</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((e, i) => (
                <tr key={i}>
                  <td>
                    <span
                      className="row-status"
                      style={{
                        background:
                          e.status === 'failed' ? 'var(--err)' :
                          e.status === 'superseded' ? 'var(--paused)' :
                          'var(--ok)',
                      }}
                    />
                  </td>
                  <td>
                    <span className="mono" style={{ color: 'var(--accent-ink)' }}>{e.revision}</span>
                    {e.current && (
                      <span className="chip info" style={{ marginLeft: 8 }}>current</span>
                    )}
                  </td>
                  <td className="mono" style={{ fontSize: 11.5 }}>{e.action || '—'}</td>
                  <td className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>{e.appVersion || '—'}</td>
                  <td>
                    <StatusChip status={e.status === 'deployed' ? 'healthy' : e.status === 'failed' ? 'failing' : 'paused'} label={e.status} />
                  </td>
                  <td className="ago">{e.timestamp ? formatEventTime(e.timestamp) : '—'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
        {data?.note && (
          <div
            className="mono"
            style={{
              padding: '8px 14px',
              fontSize: 11,
              color: 'var(--ink-3)',
              borderTop: '1px solid var(--line)',
            }}
          >
            note: {data.note}
          </div>
        )}
      </div>
    </div>
  )
}

function EventsTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppEvents(app)
  const events = data?.events ?? []

  return (
    <div style={{ padding: 18 }}>
      <div className="panel">
        <div className="panel-head">
          <div className="panel-title">
            <span className="lab">Events</span>Reconciliation timeline
          </div>
          <div className="panel-actions">
            <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
              {events.length > 0 ? `${events.length} events · live` : ''}
            </span>
          </div>
        </div>
        <div className="panel-body">
          {isLoading && events.length === 0 && <LoadingState label="Loading events…" />}
          {error && <ErrorState message={error.message} />}
          {!isLoading && !error && events.length === 0 && (
            <EmptyState
              title="No events"
              hint="Flux controllers haven't emitted Events for this resource recently. Trigger a reconcile to generate one."
            />
          )}
          {events.length > 0 && (
            <div className="timeline">
              {events.map((e) => (
                <div key={e.id} className={`tl-item ${e.kind}`}>
                  <div className="tl-meta">
                    <span>{formatEventTime(e.t)}</span>
                    <span>{e.source || 'flux'}</span>
                    <span
                      style={{
                        textTransform: 'uppercase',
                        color:
                          e.kind === 'err' ? 'var(--err)' :
                          e.kind === 'warn' ? 'var(--warn)' :
                          'var(--ok)',
                      }}
                    >
                      {e.reason}
                    </span>
                  </div>
                  <div className="tl-msg">{e.message}</div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function formatEventTime(rfc3339: string): string {
  if (!rfc3339) return ''
  try {
    return new Date(rfc3339).toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return rfc3339
  }
}

function KV({ k, v }: { k: string; v: ReactNode }) {
  return (
    <div
      style={{
        display: 'flex',
        justifyContent: 'space-between',
        borderBottom: '1px solid var(--line)',
        padding: '6px 0',
      }}
    >
      <span
        className="mono"
        style={{
          fontSize: 10.5,
          color: 'var(--ink-3)',
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
        }}
      >
        {k}
      </span>
      <span>{v}</span>
    </div>
  )
}
