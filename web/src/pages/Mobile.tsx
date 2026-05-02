import type { ComponentType, SVGProps } from 'react'
import { useApplications } from '@/lib/queries'
import type { Application } from '@/lib/types'
import { Ic } from '@/components/Icons'

interface Tab {
  label: string
  Icon: ComponentType<SVGProps<SVGSVGElement>> | null
  active?: boolean
}

const TABS: Tab[] = [
  { label: 'Incidents', Icon: Ic.alert, active: true },
  { label: 'Fleet', Icon: Ic.cluster },
  { label: 'Apps', Icon: Ic.app },
  { label: 'Me', Icon: null },
]

export function MobilePage() {
  const { data, isLoading } = useApplications()
  const apps = data?.applications ?? []
  const incidents = apps.filter((a) => a.status === 'failing' || a.status === 'degraded')
  const firing = incidents.filter((a) => a.status === 'failing').length

  return (
    <div style={{ padding: '24px 0', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 16 }}>
      <div style={{ textAlign: 'center', maxWidth: 520 }}>
        <h1 className="page-title" style={{ justifyContent: 'center' }}>On-call view</h1>
        <div className="page-sub">
          Designed for one-handed triage on a 4am pager. Get to the failing thing in 2 taps.
        </div>
      </div>
      <div className="mobile-shell">
        <div
          className="mono"
          style={{
            height: 28,
            background: 'var(--bg-2)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '0 18px',
            fontSize: 11,
          }}
        >
          <span>{new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
          <span>YAFU</span>
          <span>● ●●●●</span>
        </div>
        <div style={{ flex: 1, overflow: 'auto' }}>
          <div style={{ padding: 16, borderBottom: '1px solid var(--line)' }}>
            <div
              className="mono"
              style={{ fontSize: 10, color: 'var(--ink-3)', textTransform: 'uppercase', letterSpacing: '0.08em' }}
            >
              Active incidents
            </div>
            <div style={{ fontSize: 28, fontWeight: 600, marginTop: 6, letterSpacing: '-0.02em' }}>
              {isLoading ? '…' : `${firing} firing`}
            </div>
            <div className="mono" style={{ fontSize: 11, color: 'var(--ink-3)', marginTop: 2 }}>
              {incidents.length} unhealthy across registered clusters
            </div>
          </div>

          {incidents.length === 0 && !isLoading && (
            <div
              style={{
                padding: 24,
                textAlign: 'center',
                color: 'var(--ink-3)',
                fontSize: 12.5,
              }}
            >
              All clear. Nothing failing across your fleet.
            </div>
          )}

          {incidents.map((i) => (
            <IncidentCard key={i.id} app={i} />
          ))}
        </div>
        <div style={{ height: 56, borderTop: '1px solid var(--line)', display: 'flex', background: 'var(--bg-2)' }}>
          {TABS.map(({ label, Icon, active }) => (
            <div
              key={label}
              style={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 3,
                color: active ? 'var(--accent-ink)' : 'var(--ink-3)',
              }}
            >
              {Icon ? (
                <Icon />
              ) : (
                <div style={{ width: 18, height: 18, borderRadius: '50%', background: 'var(--accent)' }} />
              )}
              <span className="mono" style={{ fontSize: 9.5 }}>{label}</span>
            </div>
          ))}
        </div>
      </div>
      <div className="mono" style={{ fontSize: 11, color: 'var(--ink-3)' }}>
        live data · actions land in v0.2
      </div>
    </div>
  )
}

function IncidentCard({ app }: { app: Application }) {
  const tone: 'err' | 'warn' = app.status === 'failing' ? 'err' : 'warn'
  return (
    <div style={{ padding: 16, borderBottom: '1px solid var(--line)' }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 10 }}>
        <span
          style={{
            width: 4,
            height: 38,
            borderRadius: 2,
            background: tone === 'err' ? 'var(--err)' : 'var(--warn)',
          }}
        />
        <div style={{ flex: 1 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <span className={`chip ${tone}`} style={{ height: 16 }}>
              {tone === 'err' ? 'P1' : 'P3'}
            </span>
            <span style={{ fontWeight: 600, fontSize: 14 }}>{app.name}</span>
          </div>
          <div className="mono" style={{ fontSize: 11, color: 'var(--ink-3)', marginTop: 4 }}>
            {app.cluster} / {app.ns} · {app.age}
          </div>
        </div>
      </div>
      {app.message && (
        <p
          className="mono"
          style={{
            fontSize: 11.5,
            color: 'var(--ink-2)',
            marginTop: 10,
            padding: '8px 10px',
            background: 'var(--bg-2)',
            border: '1px solid var(--line)',
            borderRadius: 4,
            lineHeight: 1.5,
          }}
        >
          {app.message}
        </p>
      )}
      <div style={{ display: 'flex', gap: 6, marginTop: 10 }}>
        <button className="btn" style={{ flex: 1, justifyContent: 'center' }} disabled title="v0.2">
          Reconcile
        </button>
        <button className="btn" style={{ flex: 1, justifyContent: 'center' }} disabled title="v0.2">
          Suspend
        </button>
        <button className="btn primary" style={{ flex: 1, justifyContent: 'center' }} disabled title="v0.2">
          Open
        </button>
      </div>
    </div>
  )
}
