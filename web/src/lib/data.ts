// Placeholder data for AppDetail tabs not yet wired to the live backend
// (Resource tree, Diff, Events). The Cluster / Application / Source / Alert
// mocks were removed when the backend started serving real data — the
// Fleet and Applications pages query /api/v1/* via lib/queries.ts.

import type { DiffSide, ResourceNode, TimelineEvent } from './types'

export const RESOURCE_TREE: ResourceNode = {
  name: 'checkout-api', kind: 'Kustomization', status: 'failing',
  children: [
    { name: 'checkout', kind: 'Namespace', status: 'healthy' },
    { name: 'checkout-config', kind: 'ConfigMap', status: 'healthy' },
    { name: 'checkout-secrets', kind: 'Secret', status: 'healthy' },
    { name: 'checkout-api', kind: 'Deployment', status: 'failing', children: [
      { name: 'checkout-api-7d9c4b', kind: 'ReplicaSet', status: 'failing', children: [
        { name: 'checkout-api-7d9c4b-x4k2', kind: 'Pod', status: 'failing' },
        { name: 'checkout-api-7d9c4b-q9j1', kind: 'Pod', status: 'failing' },
      ] },
    ] },
    { name: 'checkout-api', kind: 'Service', status: 'healthy' },
    { name: 'checkout-api', kind: 'HorizontalPodAutoscaler', status: 'healthy' },
    { name: 'checkout-api-ingress', kind: 'Ingress', status: 'healthy' },
    { name: 'checkout-api', kind: 'ServiceMonitor', status: 'healthy' },
  ],
}

export const EVENTS: TimelineEvent[] = [
  { t: '14:32:08', who: 'flux-system', kind: 'err', msg: 'kustomize build failed: image overrides/values.yaml not found' },
  { t: '14:32:00', who: 'flux-system', kind: 'warn', msg: 'reconciliation triggered by source revision: main@7f3c1d9' },
  { t: '14:30:14', who: '@maria', kind: 'ok', msg: 'force-reconcile triggered via UI' },
]

export const DIFF: DiffSide[] = [
  { side: 'desired', lines: [
    { n: 14, t: '      image: ghcr.io/acme/checkout:1.8.0', cls: 'del' },
  ] },
  { side: 'live', lines: [
    { n: 14, t: '      image: ghcr.io/acme/checkout:1.7.4', cls: 'add' },
  ] },
]
