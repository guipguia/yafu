import { StatusChip } from './StatusChip'
import { Sparkline } from './Sparkline'

interface CardCluster {
  id: string
  name: string
  region: string
  env: string
  status: string
  apps: number
  ready: number
  failing: number
  suspended: number
  sources: number
  version: string
  spark: number[]
}

interface Props {
  c: CardCluster
  active?: boolean
  onClick?: () => void
}

export function ClusterCard({ c, active, onClick }: Props) {
  const overall: 'ok' | 'warn' | 'err' =
    c.failing > 0 ? (c.status === 'failing' || c.status === 'unreachable' ? 'err' : 'warn') : 'ok'
  const other = c.apps - c.ready - c.failing - c.suspended
  return (
    <div className={`cluster-card ${active ? 'active' : ''}`} onClick={onClick}>
      <div className="top">
        <div>
          <div className="name">{c.name}</div>
          {c.region && <div className="region">{c.region}</div>}
        </div>
        <StatusChip status={c.status} />
      </div>
      <div className="health">
        <div className="h-cell ok">
          <div className="n tnum">{c.ready}</div>
          <div className="l">Ready</div>
        </div>
        <div className="h-cell err">
          <div className="n tnum">{c.failing}</div>
          <div className="l">Failing</div>
        </div>
        <div className="h-cell warn">
          <div className="n tnum">{Math.max(0, other)}</div>
          <div className="l">Other</div>
        </div>
        <div className="h-cell paused">
          <div className="n tnum">{c.suspended}</div>
          <div className="l">Susp.</div>
        </div>
      </div>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginTop: 6,
        }}
      >
        <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
          {c.apps} apps · {c.sources} sources{c.version ? ` · flux ${c.version}` : ''}
        </span>
        {c.env && (
          <span className="mono" style={{ fontSize: 10.5, color: 'var(--ink-3)' }}>
            {c.env}
          </span>
        )}
      </div>
      {c.spark.length > 1 && (
        <div className="spark">
          <Sparkline data={c.spark} status={overall} />
        </div>
      )}
    </div>
  )
}
