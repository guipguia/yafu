import { useQuery } from '@tanstack/react-query'
import { fetchJSON } from './api'
import type {
  AlertsResponse,
  ApplicationsResponse,
  ClustersResponse,
  EventsResponse,
  SourcesResponse,
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
