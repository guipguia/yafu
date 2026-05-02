import { useState, type ReactNode } from 'react'
import type { Application } from '@/lib/types'
import { StatusChip } from '@/components/StatusChip'
import { ComingSoon } from '@/components/States'
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
            <button className="btn" disabled title="Coming in v0.2"><Ic.refresh /> Reconcile</button>
            {app.suspended ? (
              <button className="btn" disabled title="Coming in v0.2"><Ic.play /> Resume</button>
            ) : (
              <button className="btn" disabled title="Coming in v0.2"><Ic.pause /> Suspend</button>
            )}
            <button className="icon-btn" onClick={onClose}><Ic.x /></button>
          </div>
        </div>

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
          {tab === 'tree' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Resource tree" /></div>
          )}
          {tab === 'diff' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Live vs desired diff" /></div>
          )}
          {tab === 'events' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Reconciliation events" /></div>
          )}
          {tab === 'logs' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Live log tail" /></div>
          )}
          {tab === 'history' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Revision history & rollback" /></div>
          )}
          {tab === 'yaml' && (
            <div style={{ padding: 18 }}><ComingSoon feature="Rendered & source manifests" /></div>
          )}
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
