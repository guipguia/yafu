export function SettingsPage() {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">Settings</h1>
          <div className="page-sub">YAFU configuration · cluster connections · RBAC.</div>
        </div>
      </div>
      <div className="panel" style={{ padding: 24 }}>
        <div className="mono" style={{ fontSize: 11, color: 'var(--ink-3)', textAlign: 'center' }}>
          Settings page · placeholder for cluster connections, OIDC, RBAC, AI provider
          configuration.
        </div>
      </div>
    </>
  )
}
