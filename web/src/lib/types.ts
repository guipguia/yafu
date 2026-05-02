// API DTOs returned by the Go backend. Keep aligned with
// internal/api/types/types.go — these are the API contract until OpenAPI
// codegen lands.

export interface Cluster {
  id: string
  name: string
  region?: string
  env?: string
  /** healthy | degraded | failing | unreachable */
  status: string
  apps: number
  ready: number
  failing: number
  suspended: number
  sources: number
  version?: string
  reachable: boolean
  fluxInstalled: boolean
  lastError?: string
  spark?: number[]
}

export interface ClustersResponse {
  clusters: Cluster[]
}

export interface WhoamiResponse {
  subject: string
  email?: string
  name?: string
  groups?: string[]
  isAnonymous: boolean
}

export interface Application {
  id: string
  name: string
  /** Kustomization | HelmRelease */
  kind: string
  ns: string
  /** Display name of the cluster. */
  cluster: string
  /** Stable id used for filtering & API calls. */
  clusterId: string
  /** healthy | degraded | failing | progressing | paused */
  status: string
  /** Synced | OutOfSync | Progressing | Suspended */
  sync: string
  source: string
  revision: string
  age: string
  message?: string
  suspended: boolean
  replicas?: string
}

export interface ApplicationsResponse {
  applications: Application[]
  errors?: ClusterError[]
}

export interface ClusterError {
  cluster: string
  error: string
}

export interface Source {
  id: string
  name: string
  /** GitRepository | HelmRepository | OCIRepository | Bucket */
  kind: string
  ns: string
  cluster: string
  clusterId: string
  url: string
  ref?: string
  revision?: string
  /** healthy | degraded | failing | progressing */
  status: string
  interval?: string
  age: string
  message?: string
}

export interface SourcesResponse {
  sources: Source[]
  errors?: ClusterError[]
}

export interface Alert {
  id: string
  name: string
  cluster: string
  clusterId: string
  ns: string
  /** slack | pagerduty | webhook | … | "missing" */
  provider: string
  /** info | error */
  severity: string
  target?: string
  /** healthy | paused */
  status: string
  suspended: boolean
  age: string
}

export interface AlertsResponse {
  alerts: Alert[]
  errors?: ClusterError[]
}

export interface FluxEvent {
  id: string
  /** RFC3339 timestamp */
  t: string
  cluster: string
  clusterId: string
  ns: string
  /** ok | warn | err */
  kind: string
  /** Normal | Warning */
  type: string
  reason: string
  message: string
  /** "<Kind>/<name>" of the involved object */
  object: string
  source: string
}

export interface EventsResponse {
  events: FluxEvent[]
  errors?: ClusterError[]
}

export interface AppHistoryEntry {
  revision: string
  status?: string
  action?: string
  appVersion?: string
  /** RFC3339 timestamp */
  timestamp?: string
  current: boolean
}

export interface AppHistoryResponse {
  appId: string
  entries: AppHistoryEntry[]
  /** Explanation when full history isn't available (e.g. Kustomization). */
  note?: string
}

export interface TreeNode {
  group?: string
  version?: string
  kind: string
  ns?: string
  name: string
  /** ready | failing | progressing | notfound | unknown */
  status: string
  message?: string
}

export interface TreeResponse {
  appId: string
  nodes: TreeNode[]
  note?: string
}

export interface ManifestResponse {
  appId: string
  kind: string
  yaml: string
}

export interface ImageUpdate {
  id: string
  name: string
  cluster: string
  clusterId: string
  ns: string
  image: string
  latestTag?: string
  /** "semver:RANGE" | "alphabetical" | "numerical" | "" */
  policy: string
  /** ready | failing | progressing | paused */
  status: string
  age: string
  message?: string
  suspended: boolean
}

export interface ImageUpdatesResponse {
  updates: ImageUpdate[]
  errors?: ClusterError[]
}

export interface ManagedField {
  manager: string
  /** Apply | Update */
  operation: string
  /** RFC3339 */
  time?: string
  foreign: boolean
}

export interface DriftedResource {
  group?: string
  version?: string
  kind: string
  ns?: string
  name: string
  /** ready | drift | notfound | unknown */
  status: string
  managers?: ManagedField[]
}

export interface DiffResponse {
  appId: string
  resources: DriftedResource[]
  note?: string
}

// ---------- True Git-vs-cluster render diff ----------
// Returned by GET /api/v1/applications/{...}/render. The backend
// renders the application's Source (kustomize build / helm template)
// at the current revision, fetches the live counterpart from the
// cluster, and emits one Resource per rendered object plus any
// extras found on the cluster but not in source.

export type RenderResourceStatus =
  | 'in-sync'
  | 'drifted'
  | 'missing-on-cluster'
  | 'extra-on-cluster'
  | 'render-error'

export type RenderLineKind = 'context' | 'add' | 'del' | 'empty'

export interface RenderLine {
  kind: RenderLineKind
  /** 1-based line number in the desired (Git) YAML. Absent for added lines. */
  desiredLn?: number
  /** 1-based line number in the live (cluster) YAML. Absent for deleted lines. */
  liveLn?: number
  /** Single line of YAML, sans trailing newline. Empty for kind="empty". */
  text: string
}

export interface RenderHunk {
  /** Short human label like "spec.replicas, spec.template.spec.containers[0]". */
  label: string
  lines: RenderLine[]
}

export interface RenderResource {
  group?: string
  version?: string
  kind: string
  ns?: string
  name: string
  status: RenderResourceStatus
  /** Hunks are empty when status is "in-sync" or "missing-on-cluster". */
  hunks?: RenderHunk[]
  /** Stderr from kustomize build / helm template when status is "render-error". */
  renderError?: string
  /** Operation Flux would perform on next reconcile: "create" | "update" | "delete". */
  reconcileWould?: string
}

export interface RenderResponse {
  appId: string
  /** Source ref + revision the render was built from. */
  source: {
    name: string
    namespace: string
    /** GitRepository | HelmRepository | OCIRepository | Bucket */
    kind: string
    /** Branch/tag/semver constraint, when present. */
    ref?: string
    /** Resolved revision (commit SHA / chart version). */
    revision: string
    /** "kustomize build" | "helm template" */
    method: string
  }
  /** RFC3339 timestamp of when the render finished. */
  renderedAt: string
  resources: RenderResource[]
  /** Top-level error when the entire render failed (vs per-resource render-error). */
  error?: string
}

export interface PodInfo {
  ns: string
  name: string
  /** Pending | Running | Succeeded | Failed | Unknown */
  phase: string
  containers: string[]
  restarts: number
  age: string
}

export interface LogsResponse {
  appId: string
  pods: PodInfo[]
  /** "ns/name" of the pod whose logs were returned. */
  selected?: string
  container?: string
  logs?: string
  truncated?: boolean
  note?: string
}

// ---------- Drawer placeholder types (mock until v0.2 wires real data) ----------

export type EventKind = 'ok' | 'warn' | 'err'

export interface ResourceNode {
  name: string
  kind: string
  status: string
  children?: ResourceNode[]
}

export interface TimelineEvent {
  t: string
  who: string
  kind: EventKind
  msg: string
}

export interface DiffSide {
  side: 'desired' | 'live'
  lines: { n: number; t: string; cls: '' | 'add' | 'del' }[]
}
