# Installing yafu

This guide walks through a first-time install of yafu on an existing
Kubernetes cluster that is already running [FluxCD][flux]. By the end
you will have:

- yafu running in its own namespace
- The home cluster registered as a managed cluster
- The UI reachable via `kubectl port-forward` (or your own ingress)

For OIDC and operations details, see [auth-oidc.md](./auth-oidc.md)
and [operations.md](./operations.md).

## Prerequisites

- A Kubernetes cluster, v1.27+
- [FluxCD] installed (`flux check` returns clean)
- `kubectl` and `helm` configured against the cluster
- Cluster-admin (or equivalent) permissions to install the chart's
  ClusterRole/ClusterRoleBinding

## Install with Helm

The Helm chart is published to a chart repository hosted on GitHub
Pages. After the first chart release lands you can:

```sh
helm repo add yafu https://guipguia.github.io/yafu
helm repo update
helm install yafu yafu/yafu \
  --namespace yafu-system \
  --create-namespace
```

Until then (or for an unreleased build) install from the working tree:

```sh
git clone https://github.com/guipguia/yafu
cd yafu
helm install yafu ./charts/yafu \
  --namespace yafu-system \
  --create-namespace
```

The default install runs in **anonymous auth mode** — every request is
treated as authenticated with synthetic identity `anonymous`. This is
fine for evaluating yafu locally; **do not expose it on the public
network without enabling OIDC or fronting it with an authenticating
proxy**. See [auth-oidc.md](./auth-oidc.md).

## Reach the UI

```sh
kubectl -n yafu-system port-forward svc/yafu 8080:80
open http://localhost:8080
```

The first page is the Fleet view. It will show "no clusters
registered" until you complete the next step.

## Register the home cluster

yafu manages remote clusters by reading `Cluster` custom resources
(group `yafu.io/v1alpha1`). The simplest setup uses the in-cluster
ServiceAccount to manage the cluster yafu itself runs in:

```sh
kubectl apply -f examples/cluster-incluster.yaml
```

That manifest:

```yaml
apiVersion: yafu.io/v1alpha1
kind: Cluster
metadata:
  name: home
spec:
  displayName: kind-yafu-test
  region: 'local · kind'
  environment: dev
  fluxNamespace: flux-system
  connection:
    inCluster: true
```

Within a few seconds the controller probes the cluster, populates
`.status` on the CR, and the Fleet view goes green.

To register a remote cluster, supply a kubeconfig stored in a Secret
and reference it from `spec.connection.kubeconfigSecretRef`. The
controller resolves the Secret, builds a typed client per cluster,
and probes it on the same cadence.

## Sanity-check with a sample app

```sh
kubectl apply -f examples/podinfo-source.yaml
kubectl apply -f examples/podinfo-kustomization.yaml
```

Reload the Apps page: the new Kustomization appears with its source,
status, history, and inventory tree. Click into it to try
**Reconcile**, **Suspend**, and **Resume**.

## Common values overrides

```yaml
# values.yaml
auth:
  mode: oidc
  oidc:
    issuer: https://dex.example.com
    clientId: yafu
    redirectURL: https://yafu.example.com/auth/callback
    secretRef:
      name: yafu-oidc

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: yafu.example.com
      paths: [{ path: /, pathType: Prefix }]
  tls:
    - hosts: [yafu.example.com]
      secretName: yafu-tls

serviceMonitor:
  create: true

podDisruptionBudget:
  create: true

networkPolicy:
  create: true
```

Apply with:

```sh
helm upgrade --install yafu yafu/yafu \
  --namespace yafu-system \
  --values values.yaml
```

## Uninstall

```sh
helm uninstall yafu --namespace yafu-system
kubectl delete namespace yafu-system
kubectl delete crd clusters.yafu.io
```

The chart leaves the `yafu.io/v1alpha1.Cluster` CRD installed by
default; remove it explicitly if you no longer need the schema.

[flux]: https://fluxcd.io/flux/installation/
[FluxCD]: https://fluxcd.io/flux/installation/
