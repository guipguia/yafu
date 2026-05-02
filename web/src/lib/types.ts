// Re-exports of the OpenAPI-generated DTO schemas, with the same
// names the rest of the codebase has been importing. The generated
// types live in `api-types.ts` (do not edit by hand — run
// `npm run gen:types` after editing `api/openapi.yaml`).
//
// Centralising the re-exports here means consumers stay
// `import type { Application } from '@/lib/types'` regardless of
// where the schema actually lives. Replacing the generator (e.g.
// switching to a Go-reflect codegen) won't ripple through call
// sites.

import type { components } from './api-types'

type Schemas = components['schemas']

export type Cluster = Schemas['Cluster']
export type ClustersResponse = Schemas['ClustersResponse']

export type Application = Schemas['Application']
export type ApplicationsResponse = Schemas['ApplicationsResponse']

export type ClusterError = Schemas['ClusterError']

export type Source = Schemas['Source']
export type SourcesResponse = Schemas['SourcesResponse']

export type Alert = Schemas['Alert']
export type AlertsResponse = Schemas['AlertsResponse']

export type FluxEvent = Schemas['FluxEvent']
export type EventsResponse = Schemas['EventsResponse']

export type AppHistoryEntry = Schemas['AppHistoryEntry']
export type AppHistoryResponse = Schemas['AppHistoryResponse']

export type TreeNode = Schemas['TreeNode']
export type TreeResponse = Schemas['TreeResponse']

export type ManifestResponse = Schemas['ManifestResponse']

export type ImageUpdate = Schemas['ImageUpdate']
export type ImageUpdatesResponse = Schemas['ImageUpdatesResponse']

export type ManagedField = Schemas['ManagedField']
export type DriftedResource = Schemas['DriftedResource']
export type DiffResponse = Schemas['DiffResponse']

export type PodInfo = Schemas['PodInfo']
export type LogsResponse = Schemas['LogsResponse']

// Rendered Git-vs-cluster diff
export type RenderResource = Schemas['RenderResource']
export type RenderResourceStatus = NonNullable<RenderResource['status']>
export type RenderHunk = Schemas['RenderHunk']
export type RenderLine = Schemas['RenderLine']
export type RenderLineKind = NonNullable<RenderLine['kind']>
export type RenderSource = Schemas['RenderSource']
export type RenderResponse = Schemas['RenderResponse']

export type WhoamiResponse = Schemas['WhoamiResponse']
