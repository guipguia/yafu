import { useEffect, useState } from 'react'
import type { Application } from '@/lib/types'
import { useClusters } from '@/lib/queries'
import { useInvalidationStream } from '@/lib/stream'
import { Sidebar, type PageId } from '@/components/Sidebar'
import { Topbar } from '@/components/Topbar'
import { FleetPage, type FleetLayout } from '@/pages/Fleet'
import { AppsPage } from '@/pages/Apps'
import { AppDetailDrawer } from '@/pages/AppDetail'
import { SourcesPage } from '@/pages/Sources'
import { AlertsPage } from '@/pages/Alerts'
import { ActivityPage } from '@/pages/Activity'
import { ImagesPage } from '@/pages/Images'
import { MobilePage } from '@/pages/Mobile'
import { SettingsPage } from '@/pages/Settings'

export type Theme = 'light' | 'dark'
export type Density = 'compact' | 'comfortable'
export type SidebarMode = 'labeled' | 'icons' | 'collapsed'

const STORAGE_KEY = 'yafu.prefs.v1'

export interface Prefs {
  theme: Theme
  density: Density
  sidebar: SidebarMode
  page: PageId
  fleetLayout: FleetLayout
  clusterId: string | null
}

const DEFAULTS: Prefs = {
  theme: 'light',
  density: 'compact',
  sidebar: 'labeled',
  page: 'fleet',
  fleetLayout: 'cards',
  clusterId: null,
}

function loadPrefs(): Prefs {
  if (typeof window === 'undefined') return DEFAULTS
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return DEFAULTS
    return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<Prefs>) }
  } catch {
    return DEFAULTS
  }
}

function savePrefs(p: Prefs) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(p))
  } catch {
    // ignore
  }
}

export default function App() {
  const [prefs, setPrefs] = useState<Prefs>(loadPrefs)
  const [openApp, setOpenApp] = useState<Application | null>(null)
  const { data: clustersData } = useClusters()
  const clusters = clustersData?.clusters ?? []
  // Live invalidations from /api/v1/stream — when a Flux resource
  // changes, refetch the matching list/detail queries instead of
  // waiting for the polling fallback to catch up.
  useInvalidationStream()

  const update = <K extends keyof Prefs>(key: K, value: Prefs[K]) => {
    setPrefs((prev) => {
      const next = { ...prev, [key]: value }
      savePrefs(next)
      return next
    })
  }

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', prefs.theme)
    document.documentElement.setAttribute('data-density', prefs.density)
  }, [prefs.theme, prefs.density])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && openApp) setOpenApp(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [openApp])

  // Reconcile clusterId against the live cluster list. If the saved id is no
  // longer valid (cluster removed) or no cluster is selected yet, fall back
  // to the first cluster.
  useEffect(() => {
    if (clusters.length === 0) return
    const stillValid = prefs.clusterId && clusters.some((c) => c.id === prefs.clusterId)
    if (!stillValid) update('clusterId', clusters[0].id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [clusters.map((c) => c.id).join(',')])

  const cluster = clusters.find((c) => c.id === prefs.clusterId) ?? null

  const crumbs: Record<PageId, string[]> = {
    fleet: ['flux', 'Fleet'],
    apps: ['flux', cluster?.name ?? 'all', 'Applications'],
    sources: ['flux', cluster?.name ?? 'all', 'Sources'],
    alerts: ['flux', cluster?.name ?? 'all', 'Alerts'],
    events: ['flux', 'Activity'],
    images: ['flux', 'Image Updates'],
    mobile: ['flux', 'On-call view'],
    settings: ['flux', 'Settings'],
  }

  const renderPage = () => {
    switch (prefs.page) {
      case 'fleet':
        return (
          <FleetPage
            layout={prefs.fleetLayout}
            setLayout={(l) => update('fleetLayout', l)}
            clusterId={prefs.clusterId}
            pickCluster={(id) => {
              update('clusterId', id)
              update('page', 'apps')
            }}
          />
        )
      case 'apps':
        return <AppsPage onOpen={setOpenApp} />
      case 'sources':
        return <SourcesPage />
      case 'alerts':
        return <AlertsPage />
      case 'events':
        return <ActivityPage />
      case 'images':
        return <ImagesPage />
      case 'mobile':
        return <MobilePage />
      case 'settings':
        return (
          <SettingsPage
            theme={prefs.theme}
            density={prefs.density}
            sidebar={prefs.sidebar}
            onThemeChange={(v) => update('theme', v)}
            onDensityChange={(v) => update('density', v)}
            onSidebarChange={(v) => update('sidebar', v)}
          />
        )
    }
  }

  return (
    <div className="app" data-side={prefs.sidebar}>
      <Sidebar
        active={prefs.page}
        onNav={(id) => update('page', id)}
        side={prefs.sidebar}
        cluster={cluster}
      />
      <div className="main">
        <Topbar
          crumbs={crumbs[prefs.page]}
          theme={prefs.theme}
          onToggleTheme={() => update('theme', prefs.theme === 'dark' ? 'light' : 'dark')}
        />
        <div className="content">{renderPage()}</div>
      </div>

      {openApp && <AppDetailDrawer app={openApp} onClose={() => setOpenApp(null)} />}
    </div>
  )
}
