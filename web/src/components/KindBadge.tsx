const MAP: Record<string, { color: string; bg: string }> = {
  Kustomization: { color: 'var(--accent-ink)', bg: 'var(--accent-soft)' },
  HelmRelease: { color: 'var(--info)', bg: 'var(--info-soft)' },
  GitRepository: { color: 'var(--accent-ink)', bg: 'var(--accent-soft)' },
  HelmRepository: { color: 'var(--info)', bg: 'var(--info-soft)' },
  OCIRepository: { color: 'var(--warn)', bg: 'var(--warn-soft)' },
  Bucket: { color: 'var(--ink-2)', bg: 'var(--bg-3)' },
}

export function KindBadge({ kind }: { kind: string }) {
  const m = MAP[kind] ?? { color: 'var(--ink-2)', bg: 'var(--bg-3)' }
  return (
    <span
      style={{
        fontFamily: 'var(--font-mono)',
        fontSize: 9.5,
        padding: '1px 5px',
        borderRadius: 2,
        color: m.color,
        background: m.bg,
        letterSpacing: '0.05em',
        textTransform: 'uppercase',
        fontWeight: 500,
      }}
    >
      {kind}
    </span>
  )
}
