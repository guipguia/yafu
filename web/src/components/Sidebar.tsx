import { Fragment } from 'react'
import type { ComponentType, SVGProps } from 'react'
import type { Cluster } from '@/lib/types'
import { useApplications, useClusters } from '@/lib/queries'
import { Ic } from './Icons'

export type PageId =
  | 'fleet'
  | 'apps'
  | 'sources'
  | 'images'
  | 'alerts'
  | 'events'
  | 'mobile'
  | 'settings'

interface NavItem {
  id: PageId
  label: string
  icon: ComponentType<SVGProps<SVGSVGElement>>
  count?: number
  countCls?: '' | 'warn' | 'err'
  section?: string
}

interface Props {
  active: PageId
  onNav: (id: PageId) => void
  side: 'labeled' | 'icons' | 'collapsed'
  cluster: Cluster | null
  onClusterClick?: () => void
}

export function Sidebar({ active, onNav, side, cluster, onClusterClick }: Props) {
  const { data: clustersData } = useClusters()
  const { data: appsData } = useApplications()

  const clusters = clustersData?.clusters ?? []
  const apps = appsData?.applications ?? []
  const totalFailing = clusters.reduce((acc, c) => acc + c.failing, 0)
  const failingApps = apps.filter((a) => a.status === 'failing').length

  const items: NavItem[] = [
    { id: 'fleet', label: 'Fleet', icon: Ic.cluster, count: clusters.length || undefined, section: 'Overview' },
    { id: 'apps', label: 'Applications', icon: Ic.app, count: apps.length || undefined },
    { id: 'sources', label: 'Sources', icon: Ic.source, section: 'Catalog' },
    { id: 'images', label: 'Image Updates', icon: Ic.image },
    { id: 'alerts', label: 'Alerts', icon: Ic.alert },
    { id: 'events', label: 'Activity', icon: Ic.events, section: 'Operate' },
    { id: 'mobile', label: 'On-call view', icon: Ic.bell, count: failingApps || undefined, countCls: failingApps ? 'err' : '' },
    { id: 'settings', label: 'Settings', icon: Ic.settings, section: 'Admin' },
  ]

  const dotColor = !cluster
    ? 'var(--paused)'
    : cluster.status === 'unreachable' || cluster.status === 'failing'
    ? 'var(--err)'
    : cluster.status === 'degraded'
    ? 'var(--warn)'
    : 'var(--ok)'

  return (
    <aside className="side">
      <div className="side-head">
        <div className="logo">Y</div>
        <div className="brand">
          <div className="brand-name">YAFU</div>
          <div className="brand-sub">
            {clusters.length > 0
              ? `${clusters.length} cluster${clusters.length === 1 ? '' : 's'}${
                  totalFailing > 0 ? ` · ${totalFailing} failing` : ''
                }`
              : 'no clusters'}
          </div>
        </div>
      </div>
      <div className="side-cluster">
        <div className="lab">Active Cluster</div>
        <button className="cluster-pick" onClick={onClusterClick} disabled={!cluster}>
          <span
            className="dot"
            style={{
              background: dotColor,
              boxShadow: `0 0 0 3px color-mix(in oklch, ${dotColor} 25%, transparent)`,
            }}
          />
          <span className="name">{cluster ? cluster.name : 'no cluster selected'}</span>
          <span className="chev"><Ic.chev /></span>
        </button>
      </div>
      <nav className="nav">
        {items.map((it) => {
          const Icon = it.icon
          return (
            <Fragment key={it.id}>
              {it.section && <div className="nav-section">{it.section}</div>}
              <a
                className={`nav-item ${active === it.id ? 'active' : ''}`}
                onClick={(e) => { e.preventDefault(); onNav(it.id) }}
                href="#"
                title={side === 'icons' ? it.label : undefined}
              >
                <span className="ico"><Icon /></span>
                <span className="lab">{it.label}</span>
                {it.count !== undefined && (
                  <span className={`count ${it.countCls || ''}`}>{it.count}</span>
                )}
              </a>
            </Fragment>
          )
        })}
      </nav>
      <div className="side-foot">
        <div className="avatar">YA</div>
        <div className="side-foot-text" style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: 12, fontWeight: 500 }}>yafu</div>
          <div className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>v0.1 alpha</div>
        </div>
      </div>
    </aside>
  )
}
