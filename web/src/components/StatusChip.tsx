import type { ReactNode } from 'react'

const MAP: Record<string, { cls: string; text: string }> = {
  healthy: { cls: 'ok', text: 'Ready' },
  ready: { cls: 'ok', text: 'Ready' },
  Synced: { cls: 'ok', text: 'Synced' },
  ok: { cls: 'ok', text: 'Ready' },
  progressing: { cls: 'info', text: 'Progressing' },
  Progressing: { cls: 'info', text: 'Progressing' },
  degraded: { cls: 'warn', text: 'Degraded' },
  warn: { cls: 'warn', text: 'Warning' },
  failing: { cls: 'err', text: 'Failed' },
  err: { cls: 'err', text: 'Failed' },
  OutOfSync: { cls: 'err', text: 'OutOfSync' },
  paused: { cls: 'paused', text: 'Suspended' },
  Suspended: { cls: 'paused', text: 'Suspended' },
  firing: { cls: 'err', text: 'Firing' },
}

export function StatusChip({ status, label }: { status: string; label?: ReactNode }) {
  const m = MAP[status] ?? { cls: 'muted', text: status }
  return (
    <span className={`chip ${m.cls}`}>
      <span className="d" />
      {label ?? m.text}
    </span>
  )
}
