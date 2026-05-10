import type { ReactNode } from 'react'
import { useClusters, useVersion, useWhoami } from '@/lib/queries'
import type { Density, SidebarMode, Theme } from '@/App'

interface Props {
  theme: Theme
  density: Density
  sidebar: SidebarMode
  onThemeChange: (v: Theme) => void
  onDensityChange: (v: Density) => void
  onSidebarChange: (v: SidebarMode) => void
}

export function SettingsPage({
  theme,
  density,
  sidebar,
  onThemeChange,
  onDensityChange,
  onSidebarChange,
}: Props) {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">Settings</h1>
          <div className="page-sub">Identity, runtime build, fleet, and personal preferences.</div>
        </div>
      </div>

      <div className="split" style={{ gridTemplateColumns: '1fr 1fr', gap: 14 }}>
        <IdentityPanel />
        <BuildPanel />
      </div>

      <div style={{ marginTop: 14 }}>
        <PreferencesPanel
          theme={theme}
          density={density}
          sidebar={sidebar}
          onThemeChange={onThemeChange}
          onDensityChange={onDensityChange}
          onSidebarChange={onSidebarChange}
        />
      </div>

      <div style={{ marginTop: 14 }}>
        <FleetPanel />
      </div>
    </>
  )
}

function IdentityPanel() {
  const { data, isLoading, error } = useWhoami()

  return (
    <div className="panel">
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">Account</span>Identity
        </div>
        {data && !data.isAnonymous && (
          <div className="panel-actions">
            <a className="btn" href="/auth/logout">
              Sign out
            </a>
          </div>
        )}
      </div>
      <div className="panel-body" style={{ display: 'grid', gap: 0 }}>
        {isLoading && (
          <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
            loading identity…
          </span>
        )}
        {error && (
          <span className="mono" style={{ fontSize: 11.5, color: 'var(--err)' }}>
            {error.message}
          </span>
        )}
        {data && (
          <>
            <KV
              k="Subject"
              v={
                <span className="mono" style={{ fontSize: 11.5 }}>
                  {data.subject || '—'}
                </span>
              }
            />
            <KV k="Name" v={<span>{data.name || '—'}</span>} />
            <KV
              k="Email"
              v={
                <span className="mono" style={{ fontSize: 11.5 }}>
                  {data.email || '—'}
                </span>
              }
            />
            <KV
              k="Auth"
              v={
                <span className={`chip ${data.isAnonymous ? 'warn' : 'ok'}`}>
                  <span className="d" />
                  {data.isAnonymous ? 'anonymous' : 'authenticated'}
                </span>
              }
            />
            <KV k="Groups" v={<Groups groups={data.groups ?? []} />} />
          </>
        )}
      </div>
    </div>
  )
}

function Groups({ groups }: { groups: string[] }) {
  if (groups.length === 0) {
    return (
      <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
        —
      </span>
    )
  }
  return (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, justifyContent: 'flex-end' }}>
      {groups.map((g) => (
        <span
          key={g}
          className="mono"
          style={{
            fontSize: 10.5,
            padding: '1px 6px',
            borderRadius: 2,
            border: '1px solid var(--line-2)',
            color: 'var(--ink-2)',
            background: 'var(--bg-2)',
          }}
        >
          {g}
        </span>
      ))}
    </div>
  )
}

function BuildPanel() {
  const { data, isLoading, error } = useVersion()
  const version = data?.version || ''
  const commit = data?.commit || ''
  const date = data?.date || ''

  return (
    <div className="panel">
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">Build</span>yafu binary
        </div>
        <div className="panel-actions">
          <a
            className="btn"
            href="https://github.com/guipguia/yafu"
            target="_blank"
            rel="noreferrer noopener"
          >
            GitHub
          </a>
        </div>
      </div>
      <div className="panel-body" style={{ display: 'grid', gap: 0 }}>
        {isLoading && (
          <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
            loading build info…
          </span>
        )}
        {error && (
          <span className="mono" style={{ fontSize: 11.5, color: 'var(--err)' }}>
            {error.message}
          </span>
        )}
        {data && (
          <>
            <KV
              k="Version"
              v={
                <span className="mono" style={{ fontSize: 11.5, color: 'var(--accent-ink)' }}>
                  {version || 'dev'}
                </span>
              }
            />
            <KV
              k="Commit"
              v={
                <span className="mono" style={{ fontSize: 11.5 }} title={commit}>
                  {commit ? commit.slice(0, 12) : '—'}
                </span>
              }
            />
            <KV
              k="Built"
              v={
                <span className="mono" style={{ fontSize: 11.5 }}>
                  {date || '—'}
                </span>
              }
            />
          </>
        )}
      </div>
    </div>
  )
}

function PreferencesPanel({
  theme,
  density,
  sidebar,
  onThemeChange,
  onDensityChange,
  onSidebarChange,
}: Props) {
  return (
    <div className="panel">
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">UI</span>Preferences
        </div>
        <div className="panel-actions">
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            stored in localStorage · yafu.prefs.v1
          </span>
        </div>
      </div>
      <div
        className="panel-body"
        style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 14 }}
      >
        <PrefRow label="Theme">
          <Seg
            value={theme}
            options={[
              ['light', 'Light'],
              ['dark', 'Dark'],
            ]}
            onChange={onThemeChange}
            ariaLabel="Theme"
          />
        </PrefRow>
        <PrefRow label="Density">
          <Seg
            value={density}
            options={[
              ['compact', 'Compact'],
              ['comfortable', 'Comfortable'],
            ]}
            onChange={onDensityChange}
            ariaLabel="Row density"
          />
        </PrefRow>
        <PrefRow label="Sidebar">
          <Seg
            value={sidebar}
            options={[
              ['labeled', 'Labels'],
              ['icons', 'Icons'],
              ['collapsed', 'Hidden'],
            ]}
            onChange={onSidebarChange}
            ariaLabel="Sidebar mode"
          />
        </PrefRow>
      </div>
    </div>
  )
}

function PrefRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 6 }}>
      <span
        className="mono"
        style={{
          fontSize: 10.5,
          color: 'var(--ink-3)',
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
        }}
      >
        {label}
      </span>
      {children}
    </div>
  )
}

interface SegProps<T extends string> {
  value: T
  options: ReadonlyArray<readonly [T, string]>
  onChange: (v: T) => void
  ariaLabel: string
}

function Seg<T extends string>({ value, options, onChange, ariaLabel }: SegProps<T>) {
  return (
    <div className="seg" role="radiogroup" aria-label={ariaLabel}>
      {options.map(([v, lbl]) => (
        <button
          key={v}
          type="button"
          role="radio"
          aria-checked={value === v}
          className={value === v ? 'active' : ''}
          onClick={() => onChange(v)}
        >
          {lbl}
        </button>
      ))}
    </div>
  )
}

function FleetPanel() {
  const { data, isLoading } = useClusters()
  const clusters = data?.clusters ?? []
  const reachable = clusters.filter((c) => c.reachable).length
  const fluxOK = clusters.filter((c) => c.fluxInstalled).length

  return (
    <div className="panel">
      <div className="panel-head">
        <div className="panel-title">
          <span className="lab">Fleet</span>Registered clusters
        </div>
        <div className="panel-actions">
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            {isLoading
              ? 'loading…'
              : `${clusters.length} total · ${reachable} reachable · ${fluxOK} with Flux`}
          </span>
        </div>
      </div>
      <div className="panel-body">
        {clusters.length === 0 ? (
          <p
            className="mono"
            style={{ fontSize: 11.5, color: 'var(--ink-3)', margin: 0, lineHeight: 1.65 }}
          >
            No clusters registered yet. In CRD mode, apply a <code>yafu.io/v1alpha1.Cluster</code>{' '}
            referencing a kubeconfig <code>Secret</code>. In file mode, set{' '}
            <code>--config-file</code> or <code>--kubeconfig</code> on the yafu binary.
          </p>
        ) : (
          <table className="tbl">
            <thead>
              <tr>
                <th>Cluster</th>
                <th>Env</th>
                <th>Region</th>
                <th>Reachable</th>
                <th>Flux</th>
                <th>Version</th>
              </tr>
            </thead>
            <tbody>
              {clusters.map((c) => (
                <tr key={c.id}>
                  <td className="mono" style={{ fontSize: 11.5 }}>
                    {c.name}
                  </td>
                  <td className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
                    {c.env || '—'}
                  </td>
                  <td className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>
                    {c.region || '—'}
                  </td>
                  <td>
                    <span className={`chip ${c.reachable ? 'ok' : 'err'}`}>
                      <span className="d" />
                      {c.reachable ? 'yes' : 'no'}
                    </span>
                  </td>
                  <td>
                    <span className={`chip ${c.fluxInstalled ? 'ok' : 'warn'}`}>
                      <span className="d" />
                      {c.fluxInstalled ? 'installed' : 'missing'}
                    </span>
                  </td>
                  <td className="mono" style={{ fontSize: 11, color: 'var(--ink-3)' }}>
                    {c.version || '—'}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
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
        alignItems: 'center',
        gap: 12,
        borderBottom: '1px solid var(--line)',
        padding: '8px 0',
        minHeight: 28,
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
      <span style={{ textAlign: 'right' }}>{v}</span>
    </div>
  )
}
