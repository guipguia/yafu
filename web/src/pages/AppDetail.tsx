import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import type {
  Application,
  RenderResource,
  RenderResourceStatus,
} from '@/lib/types'
import {
  useAppDiff,
  useAppEvents,
  useAppHistory,
  useAppLogs,
  useAppManifest,
  useAppRender,
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
  const closeBtnRef = useRef<HTMLButtonElement | null>(null)

  // Move focus into the drawer on open and restore it to the row that
  // opened it on close. Esc closes the drawer.
  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    closeBtnRef.current?.focus()
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => {
      window.removeEventListener('keydown', onKey)
      previouslyFocused?.focus?.()
    }
  }, [onClose])

  const busy = reconcile.isPending || suspend.isPending || resume.isPending
  const lastError = reconcile.error || suspend.error || resume.error

  const dotColor =
    app.status === 'failing' ? 'var(--err)' :
    app.status === 'degraded' ? 'var(--warn)' :
    app.status === 'paused' ? 'var(--paused)' :
    app.status === 'progressing' ? 'var(--info)' :
    'var(--ok)'

  const titleId = 'app-detail-title'

  return (
    <>
      <div className="drawer-scrim" onClick={onClose} aria-hidden="true" />
      <div className="drawer" role="dialog" aria-modal="true" aria-labelledby={titleId}>
        <div className="drawer-head">
          <div className="titles">
            <div className="kind-badge">{app.kind}</div>
            <h2 id={titleId}>
              <span
                aria-hidden="true"
                style={{ width: 8, height: 8, borderRadius: '50%', background: dotColor }}
              />
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
            <button
              className="icon-btn"
              onClick={onClose}
              aria-label="Close application details"
              ref={closeBtnRef}
            >
              <Ic.x />
            </button>
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

        <div className="tabs" role="tablist" aria-label="Application details sections">
          {TABS.map((t) => {
            const selected = tab === t.id
            return (
              <button
                key={t.id}
                role="tab"
                type="button"
                id={`app-tab-${t.id}`}
                aria-selected={selected}
                aria-controls={`app-tabpanel-${t.id}`}
                tabIndex={selected ? 0 : -1}
                className={`tab ${selected ? 'active' : ''}`}
                onClick={() => setTab(t.id)}
                onKeyDown={(e) => {
                  if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return
                  e.preventDefault()
                  const i = TABS.findIndex((x) => x.id === tab)
                  const next = e.key === 'ArrowRight'
                    ? TABS[(i + 1) % TABS.length]
                    : TABS[(i - 1 + TABS.length) % TABS.length]
                  setTab(next.id)
                  document.getElementById(`app-tab-${next.id}`)?.focus()
                }}
              >
                {t.label}
              </button>
            )
          })}
        </div>

        <div
          className="drawer-body"
          role="tabpanel"
          id={`app-tabpanel-${tab}`}
          aria-labelledby={`app-tab-${tab}`}
        >
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
  const [live, setLive] = useState(false)
  const { data, isLoading, error } = useAppLogs(app, pod, container, 200)

  const pods = data?.pods ?? []
  const logsText = data?.logs ?? ''
  const note = data?.note ?? ''
  const truncated = !!data?.truncated
  const selectedPod = pods.find((p) => `${p.ns}/${p.name}` === data?.selected)
  const containers = selectedPod?.containers ?? []
  const activePod = pod ?? data?.selected
  const activeContainer = container ?? data?.container

  const liveLines = useLogStream(live ? buildStreamURL(app, activePod, activeContainer) : null)

  // When the user toggles "live" off, drop any tail-only state.
  useEffect(() => {
    if (!live) liveLines.clear()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [live])

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
                liveLines.clear()
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
                onChange={(e) => {
                  setContainer(e.target.value || undefined)
                  liveLines.clear()
                }}
                style={{ ...selectStyle, marginLeft: 6 }}
              >
                {containers.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            )}
          </div>
          <div className="panel-actions" style={{ gap: 10 }}>
            {truncated && !live && (
              <span
                className="mono"
                style={{ fontSize: 10.5, color: 'oklch(78% 0.16 75)' }}
              >
                truncated · last 256 KiB
              </span>
            )}
            <label
              className="mono"
              style={{
                fontSize: 11.5,
                color: 'oklch(80% 0 0)',
                display: 'inline-flex',
                alignItems: 'center',
                gap: 6,
                cursor: pods.length > 0 ? 'pointer' : 'not-allowed',
                opacity: pods.length > 0 ? 1 : 0.5,
              }}
            >
              <input
                type="checkbox"
                checked={live}
                disabled={pods.length === 0}
                onChange={(e) => setLive(e.target.checked)}
                style={{ accentColor: 'oklch(72% 0.18 275)' }}
              />
              {live && <span className="pulse" />}
              live
            </label>
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
          {isLoading && !logsText && !live && (
            <span style={{ color: 'oklch(60% 0 0)' }}>Loading…</span>
          )}
          {error && !live && (
            <span style={{ color: 'oklch(70% 0.18 25)' }}>error: {error.message}</span>
          )}
          {!isLoading && !error && pods.length === 0 && (
            <span style={{ color: 'oklch(60% 0 0)' }}>
              {note || 'No pods matched this application in its inventory namespaces.'}
            </span>
          )}
          {!live && logsText}
          {!live && !logsText && pods.length > 0 && !isLoading && (
            <span style={{ color: 'oklch(60% 0 0)' }}>
              # selected pod produced no log lines yet
            </span>
          )}
          {live && (
            <>
              {liveLines.lines.length === 0 && (
                <span style={{ color: 'oklch(60% 0 0)' }}>
                  # waiting for live output…
                </span>
              )}
              {liveLines.lines.join('\n')}
              {liveLines.error && (
                <div style={{ color: 'oklch(70% 0.18 25)', marginTop: 6 }}>
                  stream: {liveLines.error}
                </div>
              )}
            </>
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

// useLogStream subscribes to /logs/stream via EventSource. Returns a
// rolling buffer of received lines (capped at 1000) plus any terminal
// error, plus a clear() to reset state when the user switches pods.
function useLogStream(url: string | null) {
  const [lines, setLines] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const linesRef = useRef<string[]>([])

  useEffect(() => {
    if (!url) return
    setError(null)
    const es = new EventSource(url)

    const append = (line: string) => {
      linesRef.current = [...linesRef.current, line].slice(-1000)
      setLines(linesRef.current)
    }

    es.onmessage = (ev) => append(ev.data)
    es.addEventListener('open', (ev: Event) => {
      const e = ev as MessageEvent
      if (e.data) append(`# ${e.data}`)
    })
    es.addEventListener('error', (ev: Event) => {
      const e = ev as MessageEvent
      if (e.data) setError(String(e.data))
    })
    es.addEventListener('close', () => {
      es.close()
    })
    es.onerror = () => {
      // Connection-level error (network, server gone). Don't overwrite
      // a more specific server-sent error message if we already have one.
      setError((cur) => cur ?? 'connection lost')
    }

    return () => {
      es.close()
    }
  }, [url])

  const clear = () => {
    linesRef.current = []
    setLines([])
    setError(null)
  }

  return { lines, error, clear }
}

function buildStreamURL(
  app: Application,
  pod: string | undefined,
  container: string | undefined,
): string | null {
  if (!pod) return null
  const base = `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name]
    .map(encodeURIComponent)
    .join('/')}/logs/stream`
  const params = new URLSearchParams({ pod })
  if (container) params.set('container', container)
  return `${base}?${params.toString()}`
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

type DiffMode = 'drift' | 'render'

function DiffTab({ app }: { app: Application }) {
  const [mode, setMode] = useState<DiffMode>('drift')
  return (
    <div className="diff-mode-wrap">
      <div className="diff-mode-bar">
        <div className="diff-mode-seg" role="tablist" aria-label="Diff mode">
          <button
            role="tab"
            type="button"
            aria-selected={mode === 'drift'}
            aria-pressed={mode === 'drift'}
            onClick={() => setMode('drift')}
          >
            <span className="pip" /> Drift
            <span className="hint-sub">field-ownership</span>
          </button>
          <button
            role="tab"
            type="button"
            aria-selected={mode === 'render'}
            aria-pressed={mode === 'render'}
            onClick={() => setMode('render')}
          >
            <span className="pip" /> Git vs cluster
            <span className="hint-sub">rendered diff</span>
          </button>
        </div>
      </div>
      {mode === 'drift' ? <DriftTab app={app} /> : <RenderDiffTab app={app} />}
    </div>
  )
}

function DriftTab({ app }: { app: Application }) {
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

// RenderDiffTab is the rendered Git-vs-cluster diff view. It hits
// /api/v1/applications/.../render via useAppRender. The backend
// returns 501 today (the stub); when the real handler lands the
// view renders the response without code changes here.
function RenderDiffTab({ app }: { app: Application }) {
  const { data, isLoading, error } = useAppRender(app)
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [layout, setLayout] = useState<'split' | 'unified'>('split')

  if (isLoading) return <RenderLoadingState />
  if (error || !data) return <RenderUnavailableState message={error?.message} />

  const resources = data.resources
  const counts = countByStatus(resources)
  const selected = resources[selectedIdx] ?? resources[0]

  return (
    <div className="render-diff">
      <div className="diff-tab-head">
        <div className="src-meta" aria-label="Source revision">
          <Ic.git aria-hidden="true" />
          <span className="src-name">
            {data.source.namespace}/{data.source.name}
          </span>
          <span className="sep">·</span>
          {data.source.ref && (
            <>
              <span>{data.source.ref}</span>
              <span className="sep">·</span>
            </>
          )}
          <span className="rev">{data.source.revision}</span>
          <span className="sep">·</span>
          <span>{data.source.method}</span>
        </div>
        <div className="stamp" aria-label="Render timestamp">
          <span className="stamp-dot" />
          rendered {humanRelative(data.renderedAt)} · as of{' '}
          {new Date(data.renderedAt).toLocaleTimeString()}
        </div>
      </div>

      <div className="diff-shell">
        <ResourceRail
          resources={resources}
          counts={counts}
          selectedIdx={selectedIdx}
          onSelect={setSelectedIdx}
        />
        {selected ? (
          <ResourcePane resource={selected} layout={layout} onLayoutChange={setLayout} />
        ) : (
          <section className="diff-main" aria-label="Resource diff">
            <EmptyState title="No resources rendered" hint="Source produced an empty manifest." />
          </section>
        )}
      </div>
    </div>
  )
}

interface RailProps {
  resources: RenderResource[]
  counts: Record<RenderResourceStatus, number>
  selectedIdx: number
  onSelect: (i: number) => void
}

function ResourceRail({ resources, counts, selectedIdx, onSelect }: RailProps) {
  const [filter, setFilter] = useState('')
  const filtered = filter
    ? resources
        .map((r, i) => ({ r, i }))
        .filter(({ r }) =>
          (r.kind + ' ' + (r.ns ?? '') + ' ' + r.name).toLowerCase().includes(filter.toLowerCase()),
        )
    : resources.map((r, i) => ({ r, i }))

  // Header summary highlights non-in-sync counts; keeps the noise
  // floor low when nothing's drifted.
  const drifted = counts['drifted'] ?? 0
  const missing = counts['missing-on-cluster'] ?? 0
  const extra = counts['extra-on-cluster'] ?? 0
  const errored = counts['render-error'] ?? 0

  const onKey = (e: React.KeyboardEvent<HTMLLIElement>, i: number) => {
    const visible = filtered.map(({ i }) => i)
    const pos = visible.indexOf(i)
    if (pos < 0) return
    let next = pos
    if (e.key === 'ArrowDown') next = Math.min(visible.length - 1, pos + 1)
    else if (e.key === 'ArrowUp') next = Math.max(0, pos - 1)
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = visible.length - 1
    else return
    e.preventDefault()
    onSelect(visible[next])
  }

  return (
    <aside className="diff-rail" aria-label="Rendered resources">
      <div className="diff-rail-head">
        <span className="lab">Resources</span>
        <span className="diff-rail-counts">
          <span className="strong">{resources.length}</span>
          {drifted > 0 && (
            <>
              <span className="sep">·</span>
              <span style={{ color: 'var(--warn)' }}>{drifted} drifted</span>
            </>
          )}
          {missing > 0 && (
            <>
              <span className="sep">·</span>
              <span style={{ color: 'var(--info)' }}>{missing} missing</span>
            </>
          )}
          {extra > 0 && (
            <>
              <span className="sep">·</span>
              <span style={{ color: 'var(--err)' }}>{extra} extra</span>
            </>
          )}
          {errored > 0 && (
            <>
              <span className="sep">·</span>
              <span style={{ color: 'var(--err)' }}>{errored} errored</span>
            </>
          )}
        </span>
      </div>
      <div className="diff-rail-search">
        <input
          type="search"
          placeholder="Filter resources…"
          aria-label="Filter resources"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
      </div>
      <ul className="res-list" role="listbox" aria-label="Rendered resources">
        {filtered.map(({ r, i }) => {
          const isSelected = i === selectedIdx
          const tone = statusTone(r.status)
          return (
            <li
              key={`${r.kind}-${r.ns ?? ''}-${r.name}-${i}`}
              role="option"
              aria-selected={isSelected}
              tabIndex={isSelected ? 0 : -1}
              onClick={() => onSelect(i)}
              onKeyDown={(e) => onKey(e, i)}
              className="res-item"
              data-status={tone}
            >
              <span className={`res-icon ${tone}`} aria-label={statusLabel(r.status)}>
                <StatusIcon status={r.status} />
              </span>
              <div className="res-meta">
                <div className="res-kind">{r.kind}</div>
                <div className="res-name">{r.name}</div>
              </div>
              <span className={`res-badge ${tone}`}>{shortStatus(r.status)}</span>
            </li>
          )
        })}
      </ul>
      <div className="res-summary" aria-label="Status summary">
        <SummaryRow tone="ok" label="in-sync" n={counts['in-sync'] ?? 0} />
        <SummaryRow tone="warn" label="drifted" n={counts['drifted'] ?? 0} />
        <SummaryRow tone="info" label="missing" n={counts['missing-on-cluster'] ?? 0} />
        <SummaryRow tone="err" label="extra" n={counts['extra-on-cluster'] ?? 0} />
      </div>
    </aside>
  )
}

function SummaryRow({ tone, label, n }: { tone: string; label: string; n: number }) {
  return (
    <div className="row">
      <span className={`dot ${tone}`} aria-hidden="true" />
      <span>{label}</span>
      <span className="n" style={{ marginLeft: 'auto' }}>{n}</span>
    </div>
  )
}

interface PaneProps {
  resource: RenderResource
  layout: 'split' | 'unified'
  onLayoutChange: (l: 'split' | 'unified') => void
}

function ResourcePane({ resource, layout, onLayoutChange }: PaneProps) {
  const tone = statusTone(resource.status)
  return (
    <section className="diff-main" aria-label="Resource diff">
      <div className={`res-header ${tone}`}>
        <span className={`kind-badge tone-${tone}`}>{resource.kind}</span>
        <h3>
          {resource.name}
          <span className="path">
            {resource.group ? `${resource.group}/` : ''}
            {resource.version ?? 'v1'} · {resource.ns ?? '—'}/{resource.name}
          </span>
        </h3>
        {resource.reconcileWould && (
          <span
            className={`hint hint-${tone}`}
            aria-label={`Reconcile would ${resource.reconcileWould} this resource`}
          >
            <span className="d" />
            reconcile would: {resource.reconcileWould}
          </span>
        )}
      </div>

      {resource.status === 'render-error' ? (
        <RenderErrorState resource={resource} />
      ) : resource.status === 'in-sync' ? (
        <InSyncState />
      ) : resource.status === 'missing-on-cluster' ? (
        <MissingState />
      ) : resource.status === 'extra-on-cluster' ? (
        <ExtraState />
      ) : (
        <DiffBody resource={resource} layout={layout} onLayoutChange={onLayoutChange} />
      )}
    </section>
  )
}

function DiffBody({ resource, layout, onLayoutChange }: PaneProps) {
  return (
    <>
      <div className="diff-toolbar">
        <div className="seg" role="group" aria-label="Diff layout">
          <button
            type="button"
            aria-pressed={layout === 'split'}
            onClick={() => onLayoutChange('split')}
          >
            Split
          </button>
          <button
            type="button"
            aria-pressed={layout === 'unified'}
            onClick={() => onLayoutChange('unified')}
          >
            Unified
          </button>
        </div>
        <span className="grow" />
        <span className="legend" aria-hidden="true">
          <span className="sw del">desired (Git)</span>
          <span className="sw add">live (Cluster)</span>
        </span>
      </div>
      <div className="diff-scroll" tabIndex={0} role="region" aria-label="YAML diff">
        {layout === 'split' ? (
          <SplitDiff resource={resource} />
        ) : (
          <UnifiedDiff resource={resource} />
        )}
      </div>
    </>
  )
}

function SplitDiff({ resource }: { resource: RenderResource }) {
  const hunks = resource.hunks ?? []
  return (
    <div className="diff-grid-2" role="presentation">
      <div className="diff-col">
        <div className="diff-col-head">
          <span style={{ color: 'var(--err)' }}>− Desired</span>
          <span style={{ color: 'var(--ink-4)' }}>·</span>
          <span className="ln-side">Git</span>
        </div>
        {hunks.map((h, hi) => (
          <div className="hunk" key={`d-${hi}`}>
            <div className="hunk-label">@@ {h.label}</div>
            {h.lines.map((l, li) => {
              // On the desired side, "add" lines (cluster-only) render
              // as the hatched empty placeholder so columns stay aligned.
              const showEmpty = l.kind === 'add'
              const showDel = l.kind === 'del'
              const cls = showEmpty ? 'empty' : showDel ? 'del' : ''
              return (
                <div
                  className={`ln ${cls}`}
                  key={`d-${hi}-${li}`}
                  tabIndex={showDel ? 0 : undefined}
                >
                  <span className="n">{showEmpty ? '·' : l.desiredLn ?? ''}</span>
                  <code>{showEmpty ? '(no line)' : l.text}</code>
                </div>
              )
            })}
          </div>
        ))}
      </div>
      <div className="diff-col">
        <div className="diff-col-head">
          <span style={{ color: 'var(--ok)' }}>+ Live</span>
          <span style={{ color: 'var(--ink-4)' }}>·</span>
          <span className="ln-side">Cluster</span>
        </div>
        {hunks.map((h, hi) => (
          <div className="hunk" key={`l-${hi}`}>
            <div className="hunk-label hunk-label-spacer">·</div>
            {h.lines.map((l, li) => {
              const showEmpty = l.kind === 'del'
              const showAdd = l.kind === 'add'
              const cls = showEmpty ? 'empty' : showAdd ? 'add' : ''
              return (
                <div
                  className={`ln ${cls}`}
                  key={`l-${hi}-${li}`}
                  tabIndex={showAdd ? 0 : undefined}
                >
                  <span className="n">{showEmpty ? '·' : l.liveLn ?? ''}</span>
                  <code>{showEmpty ? '(no line)' : l.text}</code>
                </div>
              )
            })}
          </div>
        ))}
      </div>
    </div>
  )
}

function UnifiedDiff({ resource }: { resource: RenderResource }) {
  const hunks = resource.hunks ?? []
  return (
    <div className="diff-inline">
      {hunks.map((h, hi) => (
        <div className="hunk" key={`u-${hi}`}>
          <div className="hunk-label">@@ {h.label}</div>
          {h.lines.map((l, li) => {
            if (l.kind === 'empty') return null
            const cls = l.kind === 'add' ? 'add' : l.kind === 'del' ? 'del' : ''
            return (
              <div
                className={`ln ${cls}`}
                key={`u-${hi}-${li}`}
                tabIndex={cls ? 0 : undefined}
              >
                <span className="n">{l.desiredLn ?? ''}</span>
                <span className="n2">{l.liveLn ?? ''}</span>
                <code>{l.text}</code>
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}

function InSyncState() {
  return (
    <div className="state info">
      <div className="ico-wrap" aria-hidden="true">
        <Ic.check />
      </div>
      <h4>In sync</h4>
      <p>This resource matches the rendered source byte-for-byte.</p>
    </div>
  )
}

function MissingState() {
  return (
    <div className="state info">
      <div className="ico-wrap" aria-hidden="true">
        <Ic.warn />
      </div>
      <h4>Missing on cluster</h4>
      <p>This resource is in the rendered source but hasn't been applied yet. Triggering a reconcile will create it.</p>
    </div>
  )
}

function ExtraState() {
  return (
    <div className="state warn">
      <div className="ico-wrap" aria-hidden="true">
        <Ic.warn />
      </div>
      <h4>Extra on cluster</h4>
      <p>This resource exists on the cluster but is not in the rendered source. With <code>prune: true</code>, Flux would delete it on the next reconcile.</p>
    </div>
  )
}

function RenderErrorState({ resource }: { resource: RenderResource }) {
  return (
    <div className="state">
      <div className="ico-wrap" aria-hidden="true">
        <Ic.warn />
      </div>
      <h4>Source render failed</h4>
      <p>Flux couldn't render the manifest for this resource. The diff is unavailable until the build error is resolved.</p>
      <pre className="err-output" role="alert">{resource.renderError ?? '(no output)'}</pre>
    </div>
  )
}

// RenderUnavailableState is shown when /api/v1/applications/.../render
// returns 501 (the current stub) or any other error. It's intentionally
// honest: the rendered diff isn't built yet, and the working diff lives
// in the Drift sub-tab. When the backend lands and starts returning 200,
// this state stops appearing.
function RenderUnavailableState({ message }: { message?: string }) {
  return (
    <div className="render-diff">
      <div
        className="state info"
        style={{ marginTop: 14, marginLeft: 18, marginRight: 18 }}
      >
        <div className="ico-wrap" aria-hidden="true">
          <Ic.warn />
        </div>
        <h4>Rendered diff not yet available</h4>
        <p>
          The Git-vs-cluster view requires the backend to render the source
          (<code>kustomize build</code> or <code>helm template</code>) at the
          current revision and diff every resource against the live cluster.
          That work is the next slice — see the project roadmap.
        </p>
        <p>
          For now, switch to the <strong>Drift</strong> sub-tab — it shows
          field-ownership drift against what Flux last applied, which catches
          the most common case (someone ran <code>kubectl edit</code> on a
          managed resource).
        </p>
        {message && (
          <pre className="err-output" role="alert" style={{ maxWidth: 520 }}>
            {message}
          </pre>
        )}
      </div>
    </div>
  )
}

function RenderLoadingState() {
  return (
    <div className="render-diff">
      <div
        className="state info"
        style={{ marginTop: 14, marginLeft: 18, marginRight: 18 }}
      >
        <div className="ico-wrap" aria-hidden="true">
          <Ic.refresh />
        </div>
        <h4>Rendering source…</h4>
        <p>
          <code>kustomize build</code> can take 1–10s. We'll show the diff as
          soon as the manifest is ready.
        </p>
      </div>
    </div>
  )
}

// --- helpers ---

function countByStatus(resources: RenderResource[]): Record<RenderResourceStatus, number> {
  const out: Record<RenderResourceStatus, number> = {
    'in-sync': 0,
    drifted: 0,
    'missing-on-cluster': 0,
    'extra-on-cluster': 0,
    'render-error': 0,
  }
  for (const r of resources) out[r.status]++
  return out
}

function statusTone(s: RenderResourceStatus): 'ok' | 'warn' | 'info' | 'err' {
  switch (s) {
    case 'in-sync':
      return 'ok'
    case 'drifted':
      return 'warn'
    case 'missing-on-cluster':
      return 'info'
    case 'extra-on-cluster':
    case 'render-error':
      return 'err'
  }
}

function statusLabel(s: RenderResourceStatus): string {
  switch (s) {
    case 'in-sync': return 'In sync'
    case 'drifted': return 'Drifted'
    case 'missing-on-cluster': return 'Missing on cluster'
    case 'extra-on-cluster': return 'Extra on cluster'
    case 'render-error': return 'Render error'
  }
}

function shortStatus(s: RenderResourceStatus): string {
  switch (s) {
    case 'in-sync': return 'in-sync'
    case 'drifted': return 'drifted'
    case 'missing-on-cluster': return 'missing'
    case 'extra-on-cluster': return 'extra'
    case 'render-error': return 'render-error'
  }
}

function StatusIcon({ status }: { status: RenderResourceStatus }) {
  switch (status) {
    case 'in-sync':
      return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 8.5l3.5 3 7-7" />
        </svg>
      )
    case 'drifted':
    case 'render-error':
      return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round">
          <path d="M8 2.5l6 11h-12z" />
          <path d="M8 6.5v3.5" />
          <circle cx="8" cy="11.8" r="0.6" fill="currentColor" />
        </svg>
      )
    case 'missing-on-cluster':
      return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round">
          <path d="M8 4v5" />
          <path d="M5 9l3 3 3-3" />
          <path d="M3 14h10" />
        </svg>
      )
    case 'extra-on-cluster':
      return (
        <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth={1.4} strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="6" />
          <path d="M5.5 5.5l5 5M10.5 5.5l-5 5" />
        </svg>
      )
  }
}

function humanRelative(iso: string): string {
  const t = new Date(iso).getTime()
  const diff = Math.max(0, Date.now() - t)
  const s = Math.floor(diff / 1000)
  if (s < 60) return `${s}s ago`
  const m = Math.floor(s / 60)
  if (m < 60) return `${m}m ago`
  const h = Math.floor(m / 60)
  return `${h}h ago`
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
