import { ComingSoon } from '@/components/States'

export function ImagesPage() {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Image Updates <span className="meta">v0.2</span>
          </h1>
          <div className="page-sub">ImagePolicies and ImageUpdateAutomations watching your registries.</div>
        </div>
      </div>
      <ComingSoon feature="Image automation pipeline" />
    </>
  )
}
