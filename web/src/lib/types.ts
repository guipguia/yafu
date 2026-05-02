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
  /** ready | failing | progressing */
  status: string
  age: string
  message?: string
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
