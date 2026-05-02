import { Fragment } from 'react'
import { Ic } from './Icons'

interface Props {
  crumbs: string[]
  theme: 'light' | 'dark'
  onToggleTheme: () => void
}

export function Topbar({ crumbs, theme, onToggleTheme }: Props) {
  const themeAction = theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'
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
          <input aria-label="Search apps, resources, clusters" placeholder="Search apps, resources, clusters…" />
          <span className="kbd" aria-hidden="true">
            ⌘K
          </span>
        </div>
        <button className="icon-btn" onClick={onToggleTheme} aria-label={themeAction} title={themeAction}>
          {theme === 'dark' ? <Ic.sun /> : <Ic.moon />}
        </button>
        <button className="icon-btn" aria-label="Notifications" title="Notifications" style={{ position: 'relative' }}>
          <Ic.bell />
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
        </button>
        <button className="btn primary">
          <Ic.refresh aria-hidden="true" /> Reconcile all
        </button>
      </div>
    </header>
  )
}
