import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { Ic } from './Icons'
import {
  useApplications,
  useClusters,
  useEvents,
  useReconcileAllApps,
  useSources,
} from '@/lib/queries'
import type { Application } from '@/lib/types'
import type { PageId } from './Sidebar'

interface Props {
  crumbs: string[]
  theme: 'light' | 'dark'
  onToggleTheme: () => void
  onNavigate: (page: PageId) => void
  onOpenApp: (app: Application) => void
}

export function Topbar({ crumbs, theme, onToggleTheme, onNavigate, onOpenApp }: Props) {
  const themeAction = theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
  const { data: appsData } = useApplications()
  const apps = appsData?.applications ?? []
  const reconcileAll = useReconcileAllApps()
  const [paletteOpen, setPaletteOpen] = useState(false)

  const onReconcileAll = () => {
    if (apps.length === 0) return
    reconcileAll.mutate(apps)
  }

  // Global Cmd-K / Ctrl-K opens the palette. We intercept *before* the
  // browser's default (which on macOS Chrome focuses the URL bar).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey
      if (mod && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault()
        setPaletteOpen((v) => !v)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

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
        <button
          className="search"
          type="button"
          aria-label="Open command palette"
          onClick={() => setPaletteOpen(true)}
          style={{
            background: 'var(--bg-2)',
            border: '1px solid var(--line)',
            cursor: 'text',
            textAlign: 'left',
            color: 'var(--ink-3)',
          }}
        >
          <span aria-hidden="true">
            <Ic.search />
          </span>
          <span style={{ flex: 1, fontSize: 12.5 }}>Search apps, resources, clusters…</span>
          <span className="kbd" aria-hidden="true">
            ⌘K
          </span>
        </button>
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
      {paletteOpen && (
        <CommandPalette
          onClose={() => setPaletteOpen(false)}
          onNavigate={(page) => {
            setPaletteOpen(false)
            onNavigate(page)
          }}
          onOpenApp={(app) => {
            setPaletteOpen(false)
            onOpenApp(app)
          }}
        />
      )}
    </header>
  )
}

// ---------- Command palette ----------

type CommandItem =
  | { kind: 'page'; id: PageId; label: string; hint: string }
  | { kind: 'app'; app: Application }
  | { kind: 'source'; cluster: string; ns: string; name: string; sourceKind: string }
  | { kind: 'cluster'; id: string; name: string; status: string }

interface PaletteProps {
  onClose: () => void
  onNavigate: (page: PageId) => void
  onOpenApp: (app: Application) => void
}

function CommandPalette({ onClose, onNavigate, onOpenApp }: PaletteProps) {
  const { data: appsData } = useApplications()
  const { data: sourcesData } = useSources()
  const { data: clustersData } = useClusters()
  // Memoize against the data field so `?? []` fallbacks don't break
  // `items` memoization on every render.
  const apps = useMemo(() => appsData?.applications ?? [], [appsData?.applications])
  const sources = useMemo(() => sourcesData?.sources ?? [], [sourcesData?.sources])
  const clusters = useMemo(() => clustersData?.clusters ?? [], [clustersData?.clusters])
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState(0)
  const inputRef = useRef<HTMLInputElement | null>(null)
  const listRef = useRef<HTMLUListElement | null>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const pages: CommandItem[] = useMemo(
    () => [
      { kind: 'page', id: 'fleet', label: 'Fleet', hint: 'cluster overview' },
      { kind: 'page', id: 'apps', label: 'Applications', hint: 'Kustomizations + HelmReleases' },
      { kind: 'page', id: 'sources', label: 'Sources', hint: 'Git / Helm / OCI / Bucket' },
      { kind: 'page', id: 'images', label: 'Image Updates', hint: 'ImagePolicies' },
      { kind: 'page', id: 'alerts', label: 'Alerts', hint: 'notifications + receivers' },
      { kind: 'page', id: 'events', label: 'Activity', hint: 'cluster events stream' },
      { kind: 'page', id: 'mobile', label: 'On-call view', hint: 'incident triage' },
      { kind: 'page', id: 'settings', label: 'Settings', hint: 'identity + preferences' },
    ],
    [],
  )

  const items = useMemo<CommandItem[]>(() => {
    const all: CommandItem[] = [
      ...pages,
      ...apps.map<CommandItem>((a) => ({ kind: 'app', app: a })),
      ...sources.map<CommandItem>((s) => ({
        kind: 'source',
        cluster: s.cluster,
        ns: s.ns,
        name: s.name,
        sourceKind: s.kind,
      })),
      ...clusters.map<CommandItem>((c) => ({
        kind: 'cluster',
        id: c.id,
        name: c.name,
        status: c.status,
      })),
    ]
    if (!query.trim()) {
      // No query: show only pages and a short hint about what to type.
      return pages
    }
    const q = query.toLowerCase()
    return all.filter((item) => itemHaystack(item).toLowerCase().includes(q)).slice(0, 50)
  }, [query, apps, sources, clusters, pages])

  // Clamp cursor whenever the visible list shrinks.
  useEffect(() => {
    if (cursor >= items.length) setCursor(Math.max(0, items.length - 1))
  }, [items.length, cursor])

  // Keep the selected row in view.
  useEffect(() => {
    const li = listRef.current?.querySelector<HTMLLIElement>(`[data-idx="${cursor}"]`)
    li?.scrollIntoView({ block: 'nearest' })
  }, [cursor])

  const choose = (item: CommandItem) => {
    if (item.kind === 'page') onNavigate(item.id)
    else if (item.kind === 'app') onOpenApp(item.app)
    else if (item.kind === 'cluster') onNavigate('fleet')
    else if (item.kind === 'source') onNavigate('sources')
  }

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      style={{
        position: 'fixed',
        inset: 0,
        background: 'oklch(0% 0 0 / 0.32)',
        zIndex: 100,
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '12vh',
      }}
    >
      <div
        style={{
          width: 'min(640px, 92vw)',
          background: 'var(--panel)',
          border: '1px solid var(--line)',
          borderRadius: 'var(--radius-md)',
          boxShadow: '0 18px 48px -12px oklch(0% 0 0 / 0.35)',
          display: 'flex',
          flexDirection: 'column',
          maxHeight: '70vh',
          overflow: 'hidden',
        }}
        onMouseDown={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 10,
            padding: '12px 14px',
            borderBottom: '1px solid var(--line)',
          }}
        >
          <span aria-hidden="true" style={{ color: 'var(--ink-3)' }}>
            <Ic.search />
          </span>
          <input
            ref={inputRef}
            type="search"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setCursor(0)
            }}
            placeholder="Type to filter apps, sources, clusters, pages…"
            style={{
              flex: 1,
              border: 'none',
              outline: 'none',
              background: 'transparent',
              color: 'var(--ink)',
              fontFamily: 'var(--font-ui)',
              fontSize: 14,
            }}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.preventDefault()
                onClose()
              } else if (e.key === 'ArrowDown') {
                e.preventDefault()
                setCursor((c) => Math.min(items.length - 1, c + 1))
              } else if (e.key === 'ArrowUp') {
                e.preventDefault()
                setCursor((c) => Math.max(0, c - 1))
              } else if (e.key === 'Enter') {
                e.preventDefault()
                const it = items[cursor]
                if (it) choose(it)
              }
            }}
          />
          <span className="kbd" aria-hidden="true">
            esc
          </span>
        </div>
        {items.length === 0 ? (
          <div
            style={{
              padding: 24,
              fontSize: 12.5,
              color: 'var(--ink-3)',
              textAlign: 'center',
            }}
          >
            no matches
          </div>
        ) : (
          <ul
            ref={listRef}
            role="listbox"
            aria-label="Results"
            style={{
              listStyle: 'none',
              margin: 0,
              padding: 6,
              overflowY: 'auto',
            }}
          >
            {items.map((item, i) => (
              <li
                key={itemKey(item, i)}
                data-idx={i}
                role="option"
                aria-selected={i === cursor}
                onMouseEnter={() => setCursor(i)}
                onClick={() => choose(item)}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 10,
                  padding: '8px 10px',
                  borderRadius: 'var(--radius)',
                  cursor: 'pointer',
                  background: i === cursor ? 'var(--bg-2)' : 'transparent',
                }}
              >
                <ItemRow item={item} />
              </li>
            ))}
          </ul>
        )}
        <div
          style={{
            padding: '8px 14px',
            borderTop: '1px solid var(--line)',
            display: 'flex',
            justifyContent: 'space-between',
            fontFamily: 'var(--font-mono)',
            fontSize: 10.5,
            color: 'var(--ink-3)',
          }}
        >
          <span>↑↓ navigate · ⏎ select · esc close</span>
          <span>{items.length} results</span>
        </div>
      </div>
    </div>
  )
}

function ItemRow({ item }: { item: CommandItem }) {
  let badge: ReactNode = null
  let title = ''
  let subtitle = ''
  switch (item.kind) {
    case 'page':
      badge = <Pill tone="info">page</Pill>
      title = item.label
      subtitle = item.hint
      break
    case 'app':
      badge = (
        <Pill tone={item.app.kind === 'HelmRelease' ? 'warn' : 'info'}>
          {item.app.kind === 'HelmRelease' ? 'helm' : 'kust'}
        </Pill>
      )
      title = item.app.name
      subtitle = `${item.app.cluster} / ${item.app.ns}${item.app.source ? ' · ' + item.app.source : ''}`
      break
    case 'source':
      badge = <Pill tone="ok">{shortSourceKind(item.sourceKind)}</Pill>
      title = item.name
      subtitle = `${item.cluster} / ${item.ns}`
      break
    case 'cluster':
      badge = (
        <Pill tone={item.status === 'healthy' ? 'ok' : item.status === 'degraded' ? 'warn' : 'err'}>
          cluster
        </Pill>
      )
      title = item.name
      subtitle = item.status
      break
  }
  return (
    <>
      {badge}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{
            fontSize: 13,
            color: 'var(--ink)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {title}
        </div>
        <div
          className="mono"
          style={{
            fontSize: 10.5,
            color: 'var(--ink-3)',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
        >
          {subtitle}
        </div>
      </div>
    </>
  )
}

function Pill({ tone, children }: { tone: 'ok' | 'warn' | 'err' | 'info'; children: ReactNode }) {
  return (
    <span
      className="mono"
      style={{
        fontSize: 10,
        textTransform: 'uppercase',
        letterSpacing: '0.06em',
        padding: '2px 6px',
        borderRadius: 2,
        color:
          tone === 'ok'
            ? 'var(--ok)'
            : tone === 'warn'
              ? 'var(--warn)'
              : tone === 'err'
                ? 'var(--err)'
                : 'var(--info)',
        background:
          tone === 'ok'
            ? 'var(--ok-soft)'
            : tone === 'warn'
              ? 'var(--warn-soft)'
              : tone === 'err'
                ? 'var(--err-soft)'
                : 'var(--info-soft)',
        flex: '0 0 auto',
      }}
    >
      {children}
    </span>
  )
}

function itemHaystack(item: CommandItem): string {
  switch (item.kind) {
    case 'page':
      return `${item.label} ${item.hint}`
    case 'app':
      return `${item.app.name} ${item.app.ns} ${item.app.cluster} ${item.app.kind} ${item.app.source ?? ''}`
    case 'source':
      return `${item.name} ${item.ns} ${item.cluster} ${item.sourceKind}`
    case 'cluster':
      return `${item.name} ${item.status} cluster`
  }
}

function itemKey(item: CommandItem, fallback: number): string {
  switch (item.kind) {
    case 'page':
      return `page:${item.id}`
    case 'app':
      return `app:${item.app.id}`
    case 'source':
      return `src:${item.cluster}/${item.ns}/${item.name}`
    case 'cluster':
      return `clu:${item.id}`
    default:
      return `idx:${fallback}`
  }
}

function shortSourceKind(k: string): string {
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
