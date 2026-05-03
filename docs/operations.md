# Operating yafu

This guide covers what an operator needs after `helm install`:
observability, upgrades, backups, and the most common failure modes.

## Health endpoints

| Path | Purpose | Notes |
|------|---------|-------|
| `GET /healthz` | Liveness — the HTTP server is up | Used by the chart's `livenessProbe`. Always returns 200 once the server is listening. |
| `GET /readyz` | Readiness — the registry has at least one cluster reachable | Used by the chart's `readinessProbe`. Returns 503 until cluster discovery completes. |
| `GET /api/v1/version` | Build info | `version`, `commit`, `date` populated via `-ldflags`. |
| `GET /api/v1/whoami` | Authenticated identity + groups | Useful for verifying OIDC group claims after the redirect flow. |

## Metrics

yafu exposes Prometheus metrics on the same port as the HTTP API
(`/metrics`). Notable series:

- **`yafu_http_request_duration_seconds`** — latency histogram with
  `method`, `route`, `status` labels. Use to alert on p95/p99
  spikes per endpoint.
- **`yafu_clusters_total`**, **`yafu_clusters_ready`**,
  **`yafu_clusters_unreachable`** — gauges from the registry. Alert
  when `ready < total` for sustained periods.
- **`yafu_cluster_probe_total`** — counter of probe attempts with
  a `result` label (`ok`, `error`).
- The standard `controller-runtime` metrics on `--metrics-addr`
  (port 8081 by default) when running in CRD mode.

To scrape with prometheus-operator, enable the ServiceMonitor:

```yaml
serviceMonitor:
  create: true
  interval: 30s
  honorLabels: true
  labels:
    release: prometheus  # match your Prometheus's serviceMonitorSelector
```

`honorLabels: true` is recommended — yafu emits per-cluster metrics
labelled with the **remote** cluster name, and you don't want
Prometheus's local scrape labels to overwrite them.

## Tracing

yafu emits OTLP HTTP spans for incoming HTTP requests, the rendered
Git-vs-cluster diff pipeline, per-cluster fan-out goroutines on list
endpoints, and mutations. Disabled by default. Enable with:

```yaml
tracing:
  endpoint: http://otel-collector.observability:4318
  insecure: true       # cluster-internal collectors typically use HTTP
  sampleRate: 0.1      # head-based sampling
```

When `endpoint` is empty the tracer provider stays at the no-op
default and there is no exporter overhead.

## Audit log

Every privileged action (mutations, OIDC login, RBAC denials)
produces one JSON line on stdout, written by `internal/audit`. The
schema is intentionally flat so a sidecar or log forwarder
(Fluent Bit, Vector, ...) can ship it directly to a SIEM.

```json
{
  "time": "2026-05-03T11:14:09.121Z",
  "actor": "alice@acme.example",
  "groups": ["sre-oncall"],
  "verb": "reconcile",
  "cluster": "prod-eu-west",
  "resource": "kustomization/flux-system/podinfo",
  "result": "allow",
  "request_id": "01HX..."
}
```

There is no separate audit log file — capturing it from container
stdout is the supported pattern.

## Upgrades

yafu follows semantic versioning. Patch releases are drop-in
upgrades; minor releases may introduce new optional values. Check the
[CHANGELOG](../CHANGELOG.md) before upgrading.

```sh
helm repo update
helm upgrade yafu yafu/yafu \
  --namespace yafu-system \
  --reuse-values \
  --version <new-version>
```

Breaking changes — chart schema or CRD changes — will be called out
in CHANGELOG.md under a **Breaking** heading and mirrored in the
GitHub release notes.

## Backups

yafu itself is stateless. The only state it owns is the
**`yafu.io/v1alpha1.Cluster` CRs** that describe registered clusters
plus the Secrets they reference. A simple backup is:

```sh
kubectl get clusters.yafu.io -o yaml > yafu-clusters.yaml
kubectl -n yafu-system get secrets \
  -l app.kubernetes.io/managed-by=yafu \
  -o yaml > yafu-secrets.yaml
```

If you use a cluster backup tool (Velero, kasten, ...), back up the
namespace plus cluster-scoped `clusters.yafu.io` resources.

## Troubleshooting

### A cluster shows `NotReady` in the Fleet view

1. `kubectl get cluster.yafu.io <name> -o yaml` — check
   `.status.conditions`. The probe records `Reason` and `Message`.
2. Common causes:
   - Kubeconfig Secret missing or its `kubeconfig` key empty
   - Network unreachable from the yafu pod (egress NetworkPolicy?)
   - API-server certificate mismatch on a non-public cluster (set
     `insecureSkipTLSVerify` only as a last resort)
3. Tail the yafu pod logs and grep for the cluster name; the prober
   logs probe failures with structured fields.

### OIDC redirects loop or fail with `state mismatch`

- Check that `--oidc-redirect-url` matches **exactly** what's
  registered with the IdP (scheme, host, port, path). A trailing
  slash difference will break the round-trip.
- If you run yafu behind an HTTP-only ingress for testing, set
  `auth.oidc.secureCookie: false`. Production must keep it `true`.
- Verify the IdP's `groups` claim — set `--oidc-groups-claim` to
  whatever your IdP names the claim (`groups`, `roles`, ...).

### Mutations return 403 with anonymous auth

By design — anonymous identities have synthetic group membership and
no policy rules grant them mutation verbs. Either enable an auth
mode that produces real identities, or, for evaluation only, supply
an `--rbac-file` with a permissive rule for the synthetic identity.

### `helm install` fails with `auth.oidc.<field> is required`

The chart pre-flight rejects an OIDC config that's missing
`issuer`, `clientId`, `redirectURL`, or `secretRef.name`. Fill all
four in your values file.

### "no clusters registered" forever

In CRD mode, yafu only sees `Cluster` CRs in the cluster it runs
in. Make sure you applied at least one (see [install.md](./install.md))
and that the controller has permission to read it (`kubectl auth
can-i get clusters.yafu.io --as=system:serviceaccount:yafu-system:yafu`).
