import { Fragment } from 'react'
import { Ic } from './Icons'

interface Props {
  crumbs: string[]
  theme: 'light' | 'dark'
  onToggleTheme: () => void
}

export function Topbar({ crumbs, theme, onToggleTheme }: Props) {
  return (
    <div className="topbar">
      <div className="crumbs">
        {crumbs.map((c, i) => (
          <Fragment key={`${i}-${c}`}>
            {i > 0 && <span className="sep">/</span>}
            <span className={i === crumbs.length - 1 ? 'cur' : ''}>{c}</span>
          </Fragment>
        ))}
      </div>
      <div className="topbar-actions">
        <div className="search">
          <Ic.search />
          <input placeholder="Search apps, resources, clusters…" />
          <span className="kbd">⌘K</span>
        </div>
        <button className="icon-btn" onClick={onToggleTheme} title="Toggle theme">
          {theme === 'dark' ? <Ic.sun /> : <Ic.moon />}
        </button>
        <button className="icon-btn" title="Notifications" style={{ position: 'relative' }}>
          <Ic.bell />
          <span
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
        <button className="btn primary"><Ic.refresh /> Reconcile all</button>
      </div>
    </div>
  )
}
