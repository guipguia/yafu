import { Fragment, useEffect, useRef, useState } from 'react'
import { Ic } from './Icons'
import { useApplications, useEvents, useReconcileAllApps } from '@/lib/queries'

interface Props {
  crumbs: string[]
  theme: 'light' | 'dark'
  onToggleTheme: () => void
}

export function Topbar({ crumbs, theme, onToggleTheme }: Props) {
  const themeAction = theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
  const { data: appsData } = useApplications()
  const apps = appsData?.applications ?? []
  const reconcileAll = useReconcileAllApps()

  const onReconcileAll = () => {
    if (apps.length === 0) return
    reconcileAll.mutate(apps)
  }

  return (
    <header className="topbar" role="banner">
      <nav aria-label="Breadcrumb">
        <ol
          className="crumbs"
          style={{
            listStyle: 'none',
            display: 'flex',
            alignItems: 'center',
            gap: 6,
            margin: 0,
            padding: 0,
          }}
        >
          {crumbs.map((c, i) => {
            const isLast = i === crumbs.length - 1
            return (
              <Fragment key={`${i}-${c}`}>
                {i > 0 && (
                  <li aria-hidden="true" className="sep">
                    /
                  </li>
                )}
                <li className={isLast ? 'cur' : ''} aria-current={isLast ? 'page' : undefined}>
                  {c}
                </li>
              </Fragment>
            )
          })}
        </ol>
      </nav>
      <div className="topbar-actions">
        <div className="search" role="search">
          <span aria-hidden="true">
            <Ic.search />
          </span>
          <input
            aria-label="Search apps, resources, clusters"
            placeholder="Search apps, resources, clusters…"
          />
          <span className="kbd" aria-hidden="true">
            ⌘K
          </span>
        </div>
        <button
          className="icon-btn"
          onClick={onToggleTheme}
          aria-label={themeAction}
          title={themeAction}
        >
          {theme === 'dark' ? <Ic.sun /> : <Ic.moon />}
        </button>
        <NotificationsBell />
        <button
          className="btn primary"
          onClick={onReconcileAll}
          disabled={apps.length === 0 || reconcileAll.isPending}
          title={
            apps.length === 0
              ? 'No applications to reconcile'
              : `Reconcile every visible application (${apps.length})`
          }
        >
          <Ic.refresh aria-hidden="true" />
          {reconcileAll.isPending ? 'Reconciling…' : 'Reconcile all'}
        </button>
      </div>
    </header>
  )
}

// NotificationsBell shows recent warning/error Flux events as a
// dropdown panel. The unread dot lights up when the count is > 0;
// opening the panel marks them seen for the session (purely
// client-side — there's no server-tracked "read" state).
function NotificationsBell() {
  const { data } = useEvents()
  const events = data?.events ?? []
  const recent = events.filter((e) => e.kind === 'warn' || e.kind === 'err').slice(0, 8)

  const [open, setOpen] = useState(false)
  const [seenCount, setSeenCount] = useState(0)
  const ref = useRef<HTMLDivElement | null>(null)

  const unread = Math.max(0, recent.length - seenCount)

  // Close on outside click.
  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onDoc)
    return () => document.removeEventListener('mousedown', onDoc)
  }, [open])

  const toggle = () => {
    setOpen((v) => {
      const next = !v
      if (next) setSeenCount(recent.length)
      return next
    })
  }

  return (
    <div ref={ref} style={{ position: 'relative' }}>
      <button
        className="icon-btn"
        aria-label={`Notifications (${unread} unread)`}
        title={unread > 0 ? `${unread} new` : 'Notifications'}
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={toggle}
      >
        <Ic.bell />
        {unread > 0 && (
          <span
            aria-hidden="true"
            style={{
              position: 'absolute',
              top: 6,
              right: 6,
              width: 6,
              height: 6,
              borderRadius: '50%',
              background: 'var(--err)',
            }}
          />
        )}
      </button>
      {open && (
        <div
          role="menu"
          aria-label="Recent notifications"
          style={{
            position: 'absolute',
            top: 'calc(100% + 6px)',
            right: 0,
            width: 360,
            background: 'var(--panel)',
            border: '1px solid var(--line)',
            borderRadius: 'var(--radius-md)',
            boxShadow: '0 12px 36px -12px oklch(0% 0 0 / 0.25)',
            zIndex: 30,
            maxHeight: 480,
            overflowY: 'auto',
          }}
        >
          <div
            style={{
              padding: '8px 12px',
              borderBottom: '1px solid var(--line)',
              fontFamily: 'var(--font-mono)',
              fontSize: 10,
              textTransform: 'uppercase',
              letterSpacing: '0.08em',
              color: 'var(--ink-3)',
            }}
          >
            Recent issues
          </div>
          {recent.length === 0 ? (
            <div
              style={{
                padding: 18,
                fontSize: 12.5,
                color: 'var(--ink-3)',
                textAlign: 'center',
              }}
            >
              No warnings or errors in the last events window.
            </div>
          ) : (
            <ul style={{ listStyle: 'none', margin: 0, padding: 0 }}>
              {recent.map((e) => (
                <li
                  key={e.id}
                  style={{
                    padding: '10px 12px',
                    borderBottom: '1px solid var(--line)',
                    display: 'grid',
                    gridTemplateColumns: '6px 1fr',
                    gap: 8,
                    alignItems: 'flex-start',
                  }}
                >
                  <span
                    aria-hidden="true"
                    style={{
                      width: 6,
                      height: 6,
                      borderRadius: '50%',
                      marginTop: 4,
                      background: e.kind === 'err' ? 'var(--err)' : 'var(--warn)',
                    }}
                  />
                  <div style={{ minWidth: 0 }}>
                    <div
                      className="mono"
                      style={{
                        fontSize: 11,
                        color: e.kind === 'err' ? 'var(--err)' : 'var(--warn)',
                        textTransform: 'uppercase',
                        letterSpacing: '0.04em',
                      }}
                    >
                      {e.reason}
                      <span style={{ color: 'var(--ink-4)', margin: '0 6px' }}>·</span>
                      <span style={{ color: 'var(--ink-3)' }}>
                        {e.cluster}/{e.ns}
                      </span>
                    </div>
                    <div
                      style={{
                        fontSize: 12,
                        marginTop: 2,
                        color: 'var(--ink)',
                        wordBreak: 'break-word',
                      }}
                    >
                      {e.message}
                    </div>
                    <div
                      className="mono"
                      style={{ fontSize: 10.5, color: 'var(--ink-3)', marginTop: 2 }}
                    >
                      {e.object}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </div>
  )
}
