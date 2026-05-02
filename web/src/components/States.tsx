import type { ReactNode } from 'react'

export function LoadingState({ label = 'Loading…' }: { label?: string }) {
  return (
    <div
      style={{
        padding: 32,
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        gap: 8,
      }}
    >
      <span className="spinner" />
      <span className="mono" style={{ fontSize: 11.5, color: 'var(--ink-3)' }}>{label}</span>
    </div>
  )
}

export function ErrorState({ message }: { message: string }) {
  return (
    <div className="panel" style={{ padding: 18, borderLeft: '2px solid var(--err)' }}>
      <p style={{ color: 'var(--err)', fontSize: 13, fontWeight: 500, margin: 0 }}>API error</p>
      <p className="mono" style={{ color: 'var(--ink-3)', fontSize: 11.5, marginTop: 6 }}>
        {message}
      </p>
    </div>
  )
}

export function EmptyState({ title, hint }: { title: string; hint?: ReactNode }) {
  return (
    <div className="panel" style={{ padding: 32, textAlign: 'center' }}>
      <p style={{ color: 'var(--ink-2)', fontSize: 14, fontWeight: 500, margin: 0 }}>{title}</p>
      {hint && (
        <div style={{ color: 'var(--ink-3)', fontSize: 12.5, marginTop: 6 }}>{hint}</div>
      )}
    </div>
  )
}

export function ComingSoon({ feature }: { feature: string }) {
  return (
    <EmptyState
      title={`${feature} — coming in v0.2`}
      hint={
        <>
          The backend doesn't expose this resource yet. Track progress on the{' '}
          <span className="mono">yafu</span> roadmap.
        </>
      }
    />
  )
}
