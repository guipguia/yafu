# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog 1.1.0][keepachangelog], and
this project adheres to [Semantic Versioning 2.0.0][semver].

## [Unreleased]

Preparing the first tagged release. Items below describe what will
ship as `v0.1.0`.

### Added

- **Fleet view** — every registered cluster with health, Flux version,
  ready/total resource counts, and reachability.
- **Apps view** — unified `Kustomization` + `HelmRelease` list with
  source references, last-applied revision, suspend state, drift
  status, and per-cluster filtering.
- **Per-application detail** — revision history, inventory tree,
  live drift diff vs cluster state, rendered Git-vs-cluster diff
  (`kustomize build` / `helm template`), raw manifest, pod logs
  streamed via Server-Sent Events.
- **Sources view** — `GitRepository`, `OCIRepository`,
  `HelmRepository`, `Bucket`.
- **Alerts view** — `notification.toolkit.fluxcd.io/Alert` summary
  with provider resolution.
- **Activity view** — Kubernetes events filtered by resource.
- **Image updates view** — `image.toolkit.fluxcd.io/ImageRepository`
  status.
- **Mutations** — POST endpoints for `reconcile`, `suspend`, and
  `resume` on Kustomizations, HelmReleases, and source resources.
  Every mutation produces a JSON audit log line on stdout.
- **Multi-cluster fan-out** — list endpoints fan out to every
  registered cluster concurrently; per-cluster errors come back in a
  partial-success envelope so one slow or unreachable cluster does
  not break the request.
- **Cluster discovery** — two modes:
  - `crd` (production) — `controller-runtime` reconciler watches
    `yafu.io/v1alpha1.Cluster` CRs and resolves credentials from
    Kubernetes Secrets.
  - `file` (dev) — load a YAML config file or auto-discover
    contexts from `~/.kube/config`.
- **OpenAPI v0.1.0** at [`api/openapi.yaml`](api/openapi.yaml) with
  generated TypeScript types verified in CI.
- **Embedded React UI** — Vite, TypeScript, TanStack Query, Zustand.
  Production builds are embedded in the Go binary via `//go:embed`.
- **Helm chart** at `charts/yafu/` with hardened defaults: non-root
  user, read-only root filesystem, dropped capabilities,
  `RuntimeDefault` seccomp, optional NetworkPolicy, PodDisruptionBudget,
  and prometheus-operator ServiceMonitor.
- **End-to-end test** that spins up `kind`, installs Flux, builds and
  loads the yafu image, applies sample resources, and exercises the
  read + mutation API surface (`make e2e`).

### Security

- **Authentication** — three modes:
  - `anonymous` (default; logs a startup WARN; not for production)
  - `header` — trust `X-Forwarded-User`/`-Email`/`-Groups` from a
    front-line proxy that strips them on ingress.
  - `oidc` — native OIDC authorization-code-with-PKCE, with the
    `client_secret` and an optional cookie-signing secret loaded
    from files (preferred over flag-based passing).
- **Authorisation** — YAML policy engine matching subject (`user:`,
  `group:`, `*`), verb (`get`, `reconcile`, `suspend`, `resume`,
  `*`), and cluster glob (`prod-*`, exact, `*`). Sample policy at
  [`deploy/sample-rbac-policy.yaml`](deploy/sample-rbac-policy.yaml).
- **Audit log** — every privileged action (mutation, OIDC login,
  RBAC denial) emits one JSON line on stdout for SIEM ingestion.
- **Container** — distroless base, multi-arch (amd64/arm64), build
  provenance and SBOM attached at release time.

### Observability

- Prometheus metrics: HTTP latency histogram, cluster registry
  gauges, probe counter, plus the standard `controller-runtime`
  metrics on the manager port.
- OpenTelemetry OTLP/HTTP tracing with head-based sampling.
  Disabled until `tracing.endpoint` is set.
- Structured JSON logging via `slog`.

### Known limitations

These are explicit non-goals for `v0.1.0` and tracked for v0.2:

- `ImagePolicy` and `ImageUpdateAutomation` are not yet exposed via
  the API even though the chart's ClusterRole already permits read
  access.
- `notification.toolkit.fluxcd.io/Provider` and `Receiver` are not
  exposed.
- No manifest editing — the supported write verbs are reconcile,
  suspend, and resume only.
- The frontend polls list endpoints every 5 seconds. Informer-backed
  Server-Sent Events for live resource updates are post-v0.1; SSE
  is currently used only for pod logs.
- The end-to-end test exercises the happy path only (no failure
  injection, no multi-cluster, no RBAC edge cases).

[keepachangelog]: https://keepachangelog.com/en/1.1.0/
[semver]: https://semver.org/spec/v2.0.0.html
[Unreleased]: https://github.com/guipguia/yafu/compare/v0.0.0...HEAD
