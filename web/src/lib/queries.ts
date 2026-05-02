import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { fetchJSON } from './api'
import type {
  AlertsResponse,
  Application,
  ApplicationsResponse,
  AppHistoryResponse,
  ClustersResponse,
  DiffResponse,
  EventsResponse,
  ImageUpdatesResponse,
  LogsResponse,
  ManifestResponse,
  SourcesResponse,
  TreeResponse,
} from './types'

const POLL_MS = 5_000

export function useClusters() {
  return useQuery<ClustersResponse>({
    queryKey: ['clusters'],
    queryFn: () => fetchJSON<ClustersResponse>('/api/v1/clusters'),
    refetchInterval: POLL_MS,
  })
}

export function useApplications(clusterId?: string) {
  const path = clusterId
    ? `/api/v1/applications?cluster=${encodeURIComponent(clusterId)}`
    : '/api/v1/applications'
  return useQuery<ApplicationsResponse>({
    queryKey: ['applications', clusterId ?? 'all'],
    queryFn: () => fetchJSON<ApplicationsResponse>(path),
    refetchInterval: POLL_MS,
  })
}

export function useSources(clusterId?: string) {
  const path = clusterId
    ? `/api/v1/sources?cluster=${encodeURIComponent(clusterId)}`
    : '/api/v1/sources'
  return useQuery<SourcesResponse>({
    queryKey: ['sources', clusterId ?? 'all'],
    queryFn: () => fetchJSON<SourcesResponse>(path),
    refetchInterval: POLL_MS,
  })
}

export function useAlerts(clusterId?: string) {
  const path = clusterId
    ? `/api/v1/alerts?cluster=${encodeURIComponent(clusterId)}`
    : '/api/v1/alerts'
  return useQuery<AlertsResponse>({
    queryKey: ['alerts', clusterId ?? 'all'],
    queryFn: () => fetchJSON<AlertsResponse>(path),
    refetchInterval: POLL_MS,
  })
}

export function useImageUpdates(clusterId?: string) {
  const path = clusterId
    ? `/api/v1/image-updates?cluster=${encodeURIComponent(clusterId)}`
    : '/api/v1/image-updates'
  return useQuery<ImageUpdatesResponse>({
    queryKey: ['image-updates', clusterId ?? 'all'],
    queryFn: () => fetchJSON<ImageUpdatesResponse>(path),
    refetchInterval: POLL_MS,
  })
}

export function useEvents(clusterId?: string) {
  const path = clusterId
    ? `/api/v1/events?cluster=${encodeURIComponent(clusterId)}`
    : '/api/v1/events'
  return useQuery<EventsResponse>({
    queryKey: ['events', clusterId ?? 'all'],
    queryFn: () => fetchJSON<EventsResponse>(path),
    refetchInterval: POLL_MS,
  })
}

export function useAppLogs(app: Application | null, pod?: string, container?: string, tail?: number) {
  const path = app
    ? (() => {
        const base = `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent).join('/')}/logs`
        const params = new URLSearchParams()
        if (pod) params.set('pod', pod)
        if (container) params.set('container', container)
        if (tail) params.set('tail', String(tail))
        return params.toString() ? `${base}?${params}` : base
      })()
    : ''
  return useQuery<LogsResponse>({
    queryKey: ['logs', app?.id ?? '', pod ?? '', container ?? '', tail ?? 0],
    queryFn: () => fetchJSON<LogsResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

export function useAppDiff(app: Application | null) {
  const path = app
    ? `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent).join('/')}/diff`
    : ''
  return useQuery<DiffResponse>({
    queryKey: ['diff', app?.id ?? ''],
    queryFn: () => fetchJSON<DiffResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

export function useAppManifest(app: Application | null) {
  const path = app
    ? `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent).join('/')}/manifest`
    : ''
  return useQuery<ManifestResponse>({
    queryKey: ['manifest', app?.id ?? ''],
    queryFn: () => fetchJSON<ManifestResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

export function useAppTree(app: Application | null) {
  const path = app
    ? `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent).join('/')}/tree`
    : ''
  return useQuery<TreeResponse>({
    queryKey: ['tree', app?.id ?? ''],
    queryFn: () => fetchJSON<TreeResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

export function useAppHistory(app: Application | null) {
  const path = app
    ? `/api/v1/applications/${[app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent).join('/')}/history`
    : ''
  return useQuery<AppHistoryResponse>({
    queryKey: ['history', app?.id ?? ''],
    queryFn: () => fetchJSON<AppHistoryResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

export function useAppEvents(app: Application | null) {
  const path = app
    ? `/api/v1/events?${new URLSearchParams({
        cluster: app.clusterId,
        ns: app.ns,
        kind: app.kind,
        name: app.name,
      }).toString()}`
    : ''
  return useQuery<EventsResponse>({
    queryKey: ['events', 'app', app?.id ?? ''],
    queryFn: () => fetchJSON<EventsResponse>(path),
    refetchInterval: POLL_MS,
    enabled: app != null,
  })
}

// ---------- mutations ----------

type MutationVerb = 'reconcile' | 'suspend' | 'resume'

function appActionURL(app: Application, verb: MutationVerb): string {
  const parts = [app.clusterId, app.ns, app.kind, app.name].map(encodeURIComponent)
  return `/api/v1/applications/${parts.join('/')}/${verb}`
}

function useAppMutation(verb: MutationVerb) {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: (app: Application) =>
      fetchJSON<{ status: string; verb: string }>(appActionURL(app, verb), { method: 'POST' }),
    onSuccess: () => {
      // Force the next refresh to surface the controller's response.
      void qc.invalidateQueries({ queryKey: ['applications'] })
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    },
  })
}

export const useReconcileApp = () => useAppMutation('reconcile')
export const useSuspendApp = () => useAppMutation('suspend')
export const useResumeApp = () => useAppMutation('resume')
