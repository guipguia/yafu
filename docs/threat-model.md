# yafu threat model

Scope: yafu v0.1 deployed in-cluster behind a reverse proxy that
terminates TLS and authenticates users (oauth2-proxy, Pomerium,
ingress auth-snippet, …). This document is intentionally short — it
covers the trust boundaries, credentials, and blast radius an
enterprise security review will ask about. Update it when any of the
roadmap items in `architecture.md` lands.

## Trust boundaries

```
+---------+   TLS    +-------+   plain   +------+   per-cluster   +---------+
| Browser | -------> | Proxy | --------> | yafu | --------------> | Remote  |
| (User)  |   401    | (IdP) |  X-Fwd-*  | (Go) |   kubeconfig    | cluster |
+---------+ <------- +-------+ <-------- +------+   credentials   +---------+
```

Boundaries, in order of trust transition:

1. **User → Proxy.** Mutual: TLS provides confidentiality; the IdP
   provides identity. Anything below this line trusts what the proxy
   asserts.
2. **Proxy → yafu.** Plaintext within the cluster network. yafu
   trusts `X-Forwarded-User` / `X-Forwarded-Email` /
   `X-Forwarded-Groups` because it cannot independently verify them
   in `header` auth mode. **The proxy is responsible for stripping
   these headers from external traffic** so a client cannot inject
   them. NetworkPolicy should also restrict the yafu Service so only
   the proxy Pod can reach it.
3. **yafu → home cluster k8s API.** Service account token mounted by
   the kubelet. RBAC on the home cluster restricts it to the verbs
   the chart's ClusterRole grants (read+patch on Flux resources,
   read+update+patch on `clusters.yafu.io`, read on Secrets in
   yafu's own namespace).
4. **yafu → remote cluster k8s API.** Per-cluster kubeconfig
   credentials, sourced from a Secret on the home cluster (whose
   reference lives in the `Cluster` CR). The remote cluster's RBAC
   determines what yafu can do there — yafu has no implicit
   privilege beyond what the kubeconfig grants.

## Credential handling

| Credential | Storage | Reach | Notes |
|------------|---------|-------|-------|
| User session (cookie/JWT) | Browser only | Proxy ↔ Browser | yafu never sees the raw token. Identity arrives as headers. |
| Home cluster SA token | Pod mount via kubelet | yafu ↔ home cluster API | Standard k8s mechanism. |
| Remote cluster kubeconfig | `Secret` in yafu's namespace, mounted into the pod via the controller's `Get` | yafu memory + per-cluster `client.Client` | Never logged, never echoed to API responses. RBAC in the chart limits read to the install namespace by default. Use [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets) or [External Secrets](https://external-secrets.io) to keep them out of Git. |
| RBAC policy (`--rbac-file`) | `ConfigMap` (or whatever you mount) | yafu memory | No secrets in policy itself. |

## Blast radius

Worst-case if yafu's Pod is compromised:

- **Read:** every Flux resource (Kustomization, HelmRelease, Sources,
  Alerts, Providers) on every registered cluster, plus core Events
  and Namespaces. Sensitive data (Secrets, ConfigMaps with embedded
  credentials) is **not** in any of these — yafu does not list
  Secrets on remote clusters.
- **Write:** annotations on Flux resources and `spec.suspend`
  patches. yafu cannot create / delete / drift workloads directly —
  it only nudges Flux, which itself enforces the desired state from
  Git. An attacker can disrupt reconciliation; they cannot replace
  the manifests Flux applies.
- **Lateral movement:** any kubeconfig Secret yafu can read is a key
  to the corresponding remote cluster. RBAC scoping (per-team SA on
  the remote cluster, mapped via the kubeconfig) is the primary
  containment.

## Mitigations in v0.1

- Anonymous auth mode logs a WARN at startup; the chart's NOTES.txt
  surfaces it. Production deployments must set `--auth-mode=header`.
- All API routes live under `/api/`, mounted on a sub-mux behind the
  auth middleware. Adding a new route inherits auth automatically.
- `server.New` panics if no auth middleware is supplied — no silent
  open-by-default.
- Distroless `:nonroot` runtime image; `securityContext.runAsNonRoot:
  true`, `readOnlyRootFilesystem: true`, all caps dropped.
- Mutations always emit one audit record (success / denied / error).
- Helm chart's RBAC is least-privilege by default; widening (Secret
  reads in other namespaces, write access on home cluster) is opt-in
  via additional Roles you author yourself.

## Mitigations on the roadmap

| Risk | Mitigation | Status |
|------|-----------|--------|
| `header` mode trusts proxy headers — a misconfigured proxy is fail-open | Native OIDC verifying tokens directly | Deferred (item #2 follow-up) |
| Per-request `List` against remote APIs is slow + chatty | Informer cache per cluster; SSE invalidations | Deferred (item #4) |
| No tamper evidence on the audit log | Append to a Kubernetes-Event sink + external SIEM | Future |
| AI feature will send cluster context to a third-party LLM | Per-tenant token, redaction, opt-in | See `memory/ai_assist_feature.md` |

## Reporting

Security issues: open a private advisory on GitHub
(`Security` tab → `Report a vulnerability`). Do not file a public
issue.
