# Examples

Sample manifests used by [`docs/local-testing.md`](../docs/local-testing.md)
to spin up a working FluxCD environment for end-to-end testing.

| File | What it does |
|------|--------------|
| `podinfo-source.yaml` | Registers the public podinfo Git repo + Helm chart repository. |
| `podinfo-kustomization.yaml` | Reconciles podinfo via `kustomize build`. Creates the `podinfo` namespace and Deployment/Service/HPA. |
| `podinfo-helmrelease.yaml` | Installs the same app via Helm into the `podinfo-helm` namespace. Useful for testing HelmRelease tree + history. |
| `cluster-incluster.yaml` | A `yafu.io/v1alpha1.Cluster` CR that registers the home cluster (used in CRD mode). |

These point at `github.com/stefanprodan/podinfo` and the Flux team's
public chart repo — no credentials needed.
