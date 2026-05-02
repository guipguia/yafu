# Local end-to-end testing

This guide walks you from "no cluster" to "yafu running against a real
Flux installation with workloads to look at" in about ten minutes.

There are two paths. Pick one — they're not exclusive but you don't
need both to validate a feature:

1. **[Path A — local dev mode](#path-a--local-dev-mode)**: yafu runs
   on your laptop (`make dev`) and talks to the test cluster via
   `~/.kube/config`. Fastest iteration loop. Best for working on the
   UI, the API handlers, or anything where you're rebuilding
   constantly.

2. **[Path B — in-cluster deploy](#path-b--in-cluster-deploy)**: yafu
   runs inside the cluster as a real Pod from the helm chart, exposed
   via `kubectl port-forward`. Best for validating the chart, RBAC,
   container behaviour, and anything sensitive to running as a
   distroless non-root user.

## Prerequisites

- Docker (Desktop or engine)
- [`kind`](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) — local Kubernetes in Docker
- `kubectl`
- [`flux`](https://fluxcd.io/flux/installation/) CLI
- `helm` (only for Path B)

On Windows: install via `winget` or `scoop` — `winget install Kubernetes.kind`
etc. Run the rest in **Git Bash** or **WSL** because the Makefile uses
POSIX shell syntax.

## Step 1 — spin up the cluster

```bash
kind create cluster --name yafu-test
kubectl config use-context kind-yafu-test
kubectl get nodes
```

## Step 2 — install Flux

```bash
flux install
flux check
```

`flux check` should report all controllers ready. yafu's cluster
prober keys off the `kustomize.toolkit.fluxcd.io` API being present;
if that's there, yafu marks the cluster as `FluxInstalled`.

## Step 3 — bootstrap sample workloads

Apply the example manifests in `examples/`. They register podinfo's
public Git repo and reconcile a Kustomization plus a HelmRelease so
you have one of each kind to look at:

```bash
kubectl apply -f examples/podinfo-source.yaml
kubectl apply -f examples/podinfo-kustomization.yaml
kubectl apply -f examples/podinfo-helmrelease.yaml

flux get kustomizations -A
flux get helmreleases -A
```

Wait until `flux get kustomizations` shows `Ready: True`.

To exercise drift detection later, manually edit the live Deployment
so it diverges from Git:

```bash
kubectl -n podinfo scale deployment podinfo --replicas=6
```

## Path A — local dev mode

Already on `kind-yafu-test`? Then just run yafu:

```bash
make dev
```

This starts the Go server on `:8080` and the Vite dev server on
`:5173`. Open http://localhost:5173 — Vite proxies API calls to the
Go server.

What to verify:

- **Fleet** lists `kind-yafu-test` with `Reachable: true` and a
  Kubernetes version.
- **Apps** lists `podinfo` (Kustomization) and `podinfo-helm`
  (HelmRelease).
- Click into `podinfo`. **Overview** tab shows source/revision.
  **Resource tree** lists Deployment/Service/HPA. **Events** has a
  reconcile summary. **Logs** lists pods. **History** shows the
  current revision. **Manifests** renders the Kustomization YAML.
- **Diff** tab → **Drift** sub-tab shows field ownership. Switch to
  **Git vs cluster** to see the new view rendered against mock
  data (banner says "Preview").
- After running `kubectl scale ... --replicas=6` from step 3, watch
  the **Apps** list refresh within a few seconds — the live SSE
  invalidations land before the polling fallback. The drift status
  on the Deployment in the **Drift** sub-tab flips.
- Click **Reconcile** on the app → flux re-applies → drift is gone.
- Click **Suspend** then **Resume** — the chip and status flip
  immediately.
- **Image Updates** is empty (no ImagePolicies installed) — that's
  expected.
- **Sources** lists the GitRepository.

## Path B — in-cluster deploy

Build the image locally and load it into the kind cluster (no
registry needed). **Note** — the `--name yafu-test` flag is required;
without it `kind load` defaults to the cluster named `kind` and
fails with "no nodes found for cluster 'kind'".

```bash
make image                         # builds both :$(VERSION) and :latest
kind get clusters                  # confirm "yafu-test" is listed
kind load docker-image ghcr.io/guipguia/yafu:latest --name yafu-test
```

Install via helm. The bundled chart ships RBAC and a Cluster CRD.
Pin the image to the locally-loaded `:latest` tag and prevent kind
from trying to re-pull (it can't reach the registry from inside the
cluster network):

```bash
helm install yafu charts/yafu \
  --namespace yafu-system --create-namespace \
  --set image.repository=ghcr.io/guipguia/yafu \
  --set image.tag=latest \
  --set image.pullPolicy=Never \
  --set clusterMode=crd
```

Register the local cluster as a Cluster CR. The chart already
applied the CRD; pointing at the in-cluster ServiceAccount is the
simplest path:

```bash
kubectl apply -f examples/cluster-incluster.yaml
kubectl get clusters.yafu.io -A
```

Wait for status `Ready: True`. Then port-forward:

```bash
kubectl -n yafu-system port-forward svc/yafu 8080:8080
```

Open http://localhost:8080. The same checklist from Path A applies,
with one extra:

- **kubectl logs deploy/yafu -n yafu-system** should show structured
  JSON logs — startup messages, probe results, and audit records for
  every reconcile/suspend/resume you trigger.

## Tearing down

```bash
kind delete cluster --name yafu-test
```

That deletes everything — cluster, Flux, podinfo, yafu. Cheap to
recreate.

## Troubleshooting

- **`make dev` says "address already in use"**: `lsof -i :8080` or
  `:5173` to find the stale process and kill it.
- **`flux check` reports CRDs missing**: `flux uninstall` then
  `flux install` again — sometimes CRDs lag.
- **Apps page empty in Path A**: confirm `kubectl get
  kustomizations -A` shows entries. yafu reads from the kubeconfig
  context that's currently active when you started `make dev`.
  Switching context after start requires a restart.
- **CrashLoopBackOff in Path B**: `kubectl logs -n yafu-system
  deploy/yafu`. Most common: image not loaded into kind (`kind load
  docker-image` again).
- **The "Preview" banner on the Diff tab**: that's expected — the
  Git-vs-cluster rendering backend isn't built yet (next slice).
  Drift sub-tab returns real data.
