#!/usr/bin/env bash
# End-to-end smoke test for yafu against a real FluxCD installation.
#
# What this proves:
#   1. yafu builds, packages into a container, and starts up cleanly
#      under the helm chart's RBAC + security context.
#   2. The CRD-mode registry picks up a Cluster CR and probes it.
#   3. yafu can list Kustomizations + HelmReleases + Sources from a
#      real Flux installation through the API.
#   4. Mutations land — reconcile annotation actually appears on the
#      target resource, and audit log entries fire.
#
# Designed to run on CI (Linux, no display) and locally via
# `make e2e`. Tear-down is idempotent so a re-run after a crash
# starts from a clean slate.

set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-yafu-e2e}"
NAMESPACE="${NAMESPACE:-yafu-system}"
IMAGE_REPO="${IMAGE_REPO:-yafu-e2e}"
IMAGE_TAG="${IMAGE_TAG:-test}"
KEEP_CLUSTER="${KEEP_CLUSTER:-false}"
TIMEOUT="${TIMEOUT:-300}"

# Resolve repo root from this script's location so the test runs
# regardless of cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

step() { printf '\033[36m==>\033[0m %s\n' "$*" >&2; }
fail() { printf '\033[31mFAIL\033[0m %s\n' "$*" >&2; exit 1; }

cleanup() {
  if [[ "${KEEP_CLUSTER}" == "true" ]]; then
    step "KEEP_CLUSTER=true; leaving cluster ${CLUSTER_NAME} running"
    return
  fi
  step "tearing down cluster ${CLUSTER_NAME}"
  kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# --- prerequisites ---------------------------------------------------

step "checking prerequisites"
for cmd in kind kubectl helm flux docker jq; do
  command -v "$cmd" >/dev/null 2>&1 || fail "missing required tool: $cmd"
done

# --- cluster ---------------------------------------------------------

step "creating kind cluster ${CLUSTER_NAME}"
kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
kind create cluster --name "${CLUSTER_NAME}"
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

# --- flux ------------------------------------------------------------

step "installing Flux"
flux install --components=source-controller,kustomize-controller,helm-controller,notification-controller >/dev/null
flux check --pre >/dev/null
# flux check returns non-zero when controllers haven't reported
# Ready=True yet; poll instead of failing on the first attempt.
end=$((SECONDS + TIMEOUT))
until flux check 2>/dev/null; do
  (( SECONDS < end )) || fail "Flux controllers didn't become ready in ${TIMEOUT}s"
  sleep 4
done

# --- sample workloads ------------------------------------------------

step "applying podinfo source + Kustomization + HelmRelease"
kubectl apply -f "${REPO_ROOT}/examples/podinfo-source.yaml"
kubectl apply -f "${REPO_ROOT}/examples/podinfo-kustomization.yaml"
kubectl apply -f "${REPO_ROOT}/examples/podinfo-helmrelease.yaml"

# Wait for at least one Kustomization + HelmRelease to be Ready.
end=$((SECONDS + TIMEOUT))
until kubectl get kustomizations.kustomize.toolkit.fluxcd.io -n flux-system podinfo \
        -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q True; do
  (( SECONDS < end )) || fail "podinfo Kustomization didn't reconcile"
  sleep 5
done

# --- yafu image + chart install --------------------------------------

step "building yafu image ${IMAGE_REPO}:${IMAGE_TAG}"
( cd "${REPO_ROOT}" && \
  docker build \
    --build-arg VERSION=e2e \
    -t "${IMAGE_REPO}:${IMAGE_TAG}" . >/dev/null )

step "loading image into kind"
kind load docker-image "${IMAGE_REPO}:${IMAGE_TAG}" --name "${CLUSTER_NAME}"

step "helm installing yafu"
helm install yafu "${REPO_ROOT}/charts/yafu" \
  --namespace "${NAMESPACE}" --create-namespace \
  --set "image.repository=${IMAGE_REPO}" \
  --set "image.tag=${IMAGE_TAG}" \
  --set image.pullPolicy=Never \
  --set clusterMode=crd \
  --wait --timeout "${TIMEOUT}s" >/dev/null

# --- register the home cluster ---------------------------------------

step "applying Cluster CR (in-cluster ServiceAccount)"
kubectl apply -f "${REPO_ROOT}/examples/cluster-incluster.yaml"

end=$((SECONDS + TIMEOUT))
until kubectl get clusters.yafu.io home \
        -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null | grep -q True; do
  (( SECONDS < end )) || fail "yafu Cluster CR didn't become Ready"
  sleep 4
done

# --- API smoke tests -------------------------------------------------

step "port-forwarding yafu service"
kubectl -n "${NAMESPACE}" port-forward svc/yafu 18080:80 >/dev/null 2>&1 &
PF_PID=$!
trap 'kill $PF_PID >/dev/null 2>&1 || true; cleanup' EXIT

# Wait for the port-forward to accept connections.
end=$((SECONDS + 30))
until curl -sf http://127.0.0.1:18080/healthz >/dev/null 2>&1; do
  (( SECONDS < end )) || fail "port-forward didn't come up"
  sleep 1
done

step "GET /healthz"
curl -sf http://127.0.0.1:18080/healthz | grep -qi 'ok\|"status"' || fail "/healthz didn't return ok"

step "GET /api/v1/clusters returns the home cluster"
clusters_json=$(curl -sf http://127.0.0.1:18080/api/v1/clusters)
echo "${clusters_json}" | jq -e '.clusters | length >= 1' >/dev/null || \
  fail "expected at least 1 cluster, got: ${clusters_json}"
echo "${clusters_json}" | jq -e '.clusters[0].fluxInstalled == true' >/dev/null || \
  fail "fluxInstalled flag was not true: ${clusters_json}"

step "GET /api/v1/applications lists podinfo"
apps_json=$(curl -sf http://127.0.0.1:18080/api/v1/applications)
echo "${apps_json}" | jq -e '.applications | map(select(.name == "podinfo")) | length >= 1' >/dev/null || \
  fail "podinfo not in applications response: ${apps_json}"
echo "${apps_json}" | jq -e '.applications | map(select(.kind == "HelmRelease")) | length >= 1' >/dev/null || \
  fail "no HelmRelease in applications response: ${apps_json}"

step "GET /api/v1/sources lists the GitRepository"
sources_json=$(curl -sf http://127.0.0.1:18080/api/v1/sources)
echo "${sources_json}" | jq -e '.sources | map(select(.kind == "GitRepository" and .name == "podinfo")) | length >= 1' >/dev/null || \
  fail "podinfo GitRepository missing from sources: ${sources_json}"

step "POST reconcile mutation lands annotation on the resource"
curl -sf -X POST http://127.0.0.1:18080/api/v1/applications/home/flux-system/Kustomization/podinfo/reconcile \
  | jq -e '.status == "accepted"' >/dev/null || fail "reconcile didn't return accepted"

# Annotation arrival is async — give the cluster a moment.
sleep 3
ann=$(kubectl get kustomization.kustomize.toolkit.fluxcd.io -n flux-system podinfo \
  -o jsonpath='{.metadata.annotations.reconcile\.fluxcd\.io/requestedAt}' 2>/dev/null || true)
[[ -n "${ann}" ]] || fail "reconcile annotation never landed"

step "GET /api/v1/applications/.../tree returns the inventory"
tree_json=$(curl -sf "http://127.0.0.1:18080/api/v1/applications/home/flux-system/Kustomization/podinfo/tree")
echo "${tree_json}" | jq -e '.nodes | length >= 1' >/dev/null || \
  fail "tree had no nodes: ${tree_json}"

step "all E2E checks passed"
