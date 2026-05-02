# End-to-end tests

`run.sh` brings up a real FluxCD installation in a kind cluster,
deploys yafu via the helm chart, and asserts the API behaves
correctly end-to-end. Same path the [local-testing
walkthrough](../../docs/local-testing.md) takes a human through —
automated.

## What it covers

- yafu builds + packages into a container without warnings
- The CRD-mode registry picks up a `Cluster` CR and probes it
- `/api/v1/clusters` returns the home cluster with `fluxInstalled=true`
- `/api/v1/applications` lists both a Kustomization and a HelmRelease
- `/api/v1/sources` returns the registered GitRepository
- A reconcile mutation lands the `reconcile.fluxcd.io/requestedAt`
  annotation on the target resource
- The resource tree endpoint walks the inventory

That set is intentionally small — it's the smoke-test layer above
unit tests, not exhaustive coverage. Anything more specific belongs
in Go handler tests next to the code under test.

## Running locally

```bash
make e2e          # full test, tears down on exit
KEEP_CLUSTER=true make e2e   # leaves the kind cluster up for poking
```

Prerequisites: docker, kind, kubectl, helm, flux, jq. On Windows,
run via Git Bash (the script uses POSIX shell features).

## Running in CI

Wired as the `e2e` job in `.github/workflows/ci.yaml`. Triggers on
PRs that touch the API, render package, helm chart, or the test
itself — skipping it for docs-only changes keeps merges fast.

## Knobs

| env var         | default     | meaning                                   |
|-----------------|-------------|-------------------------------------------|
| `CLUSTER_NAME`  | `yafu-e2e`  | kind cluster name                         |
| `NAMESPACE`     | `yafu-system`| where yafu's chart installs              |
| `IMAGE_REPO`    | `yafu-e2e`  | local image name (no registry push)       |
| `IMAGE_TAG`     | `test`      | local image tag                           |
| `KEEP_CLUSTER`  | `false`     | when `true`, skip teardown                |
| `TIMEOUT`       | `300`       | seconds to wait for any single step       |
