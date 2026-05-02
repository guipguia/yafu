import { ComingSoon } from '@/components/States'

export function ActivityPage() {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Activity <span className="meta">v0.2</span>
          </h1>
          <div className="page-sub">Every change Flux observed or applied — who, what, when, where.</div>
        </div>
      </div>
      <ComingSoon feature="Forensic timeline" />
    </>
  )
}
