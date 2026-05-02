import { ComingSoon } from '@/components/States'

export function SourcesPage() {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Sources <span className="meta">v0.2</span>
          </h1>
          <div className="page-sub">
            Git repositories, Helm repositories, OCI registries and Buckets that Flux pulls from.
          </div>
        </div>
      </div>
      <ComingSoon feature="Sources view" />
    </>
  )
}
