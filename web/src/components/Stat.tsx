import type { ReactNode } from 'react'

interface Props {
  label: string
  value: ReactNode
  of?: ReactNode
  delta?: ReactNode
  tone?: '' | 'ok' | 'warn' | 'err'
  accent?: string
}

export function Stat({ label, value, of, delta, tone = '', accent }: Props) {
  return (
    <div className={`stat ${tone}`}>
      <div className="accent-bar" style={accent ? { background: accent } : undefined} />
      <div className="lab">{label}</div>
      <div className="val tnum mono">
        {value}
        {of !== undefined && <span className="of">/ {of}</span>}
      </div>
      {delta && <div className="delta">{delta}</div>}
    </div>
  )
}
