// Placeholder fixture used by the Diff tab's "Git vs cluster" mode
// until the backend rendering endpoint lands. Once the real
// /api/v1/applications/.../render handler exists, the page will
// drop this import and consume `useAppRender` directly.

import type { Application, RenderResponse } from './types'

export function mockRenderResponse(app: Application): RenderResponse {
  return {
    appId: app.id,
    source: {
      name: app.source || `${app.ns}/${app.name}`,
      namespace: app.ns,
      kind: 'GitRepository',
      ref: 'main',
      revision: app.revision || '7f3c1d9',
      method: app.kind === 'HelmRelease' ? 'helm template' : 'kustomize build',
    },
    renderedAt: new Date(Date.now() - 14_000).toISOString(),
    resources: [
      {
        kind: 'Namespace',
        name: app.name,
        status: 'in-sync',
      },
      {
        kind: 'ConfigMap',
        ns: app.name,
        name: `${app.name}-config`,
        status: 'in-sync',
      },
      {
        kind: 'Deployment',
        ns: app.name,
        name: app.name,
        status: 'drifted',
        reconcileWould: 'update',
        hunks: [
          {
            label: 'spec.replicas, spec.template.spec.containers[0]',
            lines: [
              { kind: 'context', desiredLn: 8, liveLn: 8, text: 'spec:' },
              { kind: 'del', desiredLn: 9, text: '  replicas: 4' },
              { kind: 'add', liveLn: 9, text: '  replicas: 6' },
              { kind: 'context', desiredLn: 10, liveLn: 10, text: '  selector:' },
              { kind: 'context', desiredLn: 11, liveLn: 11, text: '    matchLabels:' },
              { kind: 'context', desiredLn: 12, liveLn: 12, text: '      app: ' + app.name },
              { kind: 'context', desiredLn: 13, liveLn: 13, text: '  template:' },
              { kind: 'context', desiredLn: 14, liveLn: 14, text: '    spec:' },
              { kind: 'context', desiredLn: 15, liveLn: 15, text: '      containers:' },
              { kind: 'context', desiredLn: 16, liveLn: 16, text: '        - name: ' + app.name },
              { kind: 'del', desiredLn: 17, text: '          image: ghcr.io/stefanprodan/podinfo:6.7.1' },
              { kind: 'add', liveLn: 17, text: '          image: ghcr.io/stefanprodan/podinfo:6.6.2' },
              { kind: 'context', desiredLn: 18, liveLn: 18, text: '          ports:' },
              { kind: 'context', desiredLn: 19, liveLn: 19, text: '            - containerPort: 9898' },
              { kind: 'context', desiredLn: 20, liveLn: 20, text: '          resources:' },
              { kind: 'context', desiredLn: 21, liveLn: 21, text: '            limits:' },
              { kind: 'del', desiredLn: 22, text: '              cpu: 500m' },
              { kind: 'add', liveLn: 22, text: '              cpu: 250m' },
              { kind: 'context', desiredLn: 23, liveLn: 23, text: '              memory: 256Mi' },
              { kind: 'add', liveLn: 24, text: '          env:' },
              { kind: 'add', liveLn: 25, text: '            - name: PODINFO_UI_COLOR' },
              { kind: 'context', desiredLn: 24, liveLn: 26, text: '          livenessProbe:' },
              { kind: 'context', desiredLn: 25, liveLn: 27, text: '            httpGet:' },
              { kind: 'context', desiredLn: 26, liveLn: 28, text: '              path: /healthz' },
              { kind: 'context', desiredLn: 27, liveLn: 29, text: '              port: 9898' },
            ],
          },
        ],
      },
      {
        kind: 'HorizontalPodAutoscaler',
        ns: app.name,
        name: app.name,
        status: 'drifted',
        reconcileWould: 'update',
        hunks: [
          {
            label: 'spec.maxReplicas',
            lines: [
              { kind: 'context', desiredLn: 5, liveLn: 5, text: 'spec:' },
              { kind: 'del', desiredLn: 6, text: '  maxReplicas: 8' },
              { kind: 'add', liveLn: 6, text: '  maxReplicas: 12' },
              { kind: 'context', desiredLn: 7, liveLn: 7, text: '  minReplicas: 2' },
            ],
          },
        ],
      },
      {
        kind: 'ServiceMonitor',
        ns: app.name,
        name: app.name,
        status: 'missing-on-cluster',
        reconcileWould: 'create',
      },
      {
        kind: 'Service',
        ns: app.name,
        name: app.name,
        status: 'in-sync',
      },
      {
        kind: 'Ingress',
        ns: app.name,
        name: `${app.name}-legacy`,
        status: 'extra-on-cluster',
        reconcileWould: 'delete',
      },
      {
        kind: 'CronJob',
        ns: app.name,
        name: `${app.name}-cleanup`,
        status: 'render-error',
        renderError:
          "$ kustomize build ./apps/" + app.name + " --enable-helm\n" +
          "Error: accumulating resources: accumulation err='accumulating resources from 'cronjobs/cleanup.yaml':\n" +
          "  evalsymlink failure on '/tmp/" + app.name + "/cronjobs/cleanup.yaml': lstat: no such file or directory'\n" +
          "hint: referenced in kustomization.yaml: line 14\n" +
          "       resources:\n" +
          "         - cronjobs/cleanup.yaml      ← not found",
      },
    ],
  }
}
