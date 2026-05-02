import { ComingSoon } from '@/components/States'

export function AlertsPage() {
  return (
    <>
      <div className="page-head">
        <div>
          <h1 className="page-title">
            Alerts &amp; Notifications <span className="meta">v0.2</span>
          </h1>
          <div className="page-sub">Routes that send Flux events to Slack, PagerDuty, MS Teams, webhooks.</div>
        </div>
      </div>
      <ComingSoon feature="Alerts &amp; routing graph" />
    </>
  )
}
