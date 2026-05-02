package render

import (
	"context"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// minimalChart writes a tiny but valid Helm chart under root.
// It has one value (.Values.replicaCount) used by a Deployment
// template + a fixed Service. That gives us enough surface area to
// test value passthrough, default-namespace stamping, and
// rendering of multiple manifests.
func minimalChart(t *testing.T, root string) {
	t.Helper()
	writeFiles(t, root, map[string]string{
		"Chart.yaml": `
apiVersion: v2
name: tiny
version: 0.1.0
type: application
`,
		"values.yaml": `
replicaCount: 1
`,
		"templates/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels: { app: {{ .Release.Name }} }
  template:
    metadata: { labels: { app: {{ .Release.Name }} } }
    spec:
      containers:
        - name: app
          image: ghcr.io/example/app:1.0
`,
		"templates/service.yaml": `
apiVersion: v1
kind: Service
metadata:
  name: {{ .Release.Name }}
spec:
  selector: { app: {{ .Release.Name }} }
  ports: [ { port: 80 } ]
`,
		"templates/_helpers.tpl": `
{{- define "tiny.name" -}}
{{- .Release.Name -}}
{{- end -}}
`,
	})
}

func TestRenderHelmRelease_HappyPath(t *testing.T) {
	root := t.TempDir()
	minimalChart(t, root)

	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "podinfo", Namespace: "podinfo-ns"},
		Spec: helmv2.HelmReleaseSpec{
			Values: &apiextv1.JSON{Raw: []byte(`{"replicaCount": 3}`)},
		},
	}

	objs, err := RenderHelmRelease(context.Background(), root, hr)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("got %d objects, want 2", len(objs))
	}

	var depl, svc int
	for _, o := range objs {
		switch o.GetKind() {
		case "Deployment":
			depl++
			if o.GetName() != "podinfo" {
				t.Errorf("Deployment name = %s", o.GetName())
			}
			if o.GetNamespace() != "podinfo-ns" {
				t.Errorf("Deployment ns = %s, want podinfo-ns (default applied)", o.GetNamespace())
			}
			replicas, found, err := getNestedInt(o.Object, "spec", "replicas")
			if err != nil || !found {
				t.Fatalf("replicas not found: %v", err)
			}
			if replicas != 3 {
				t.Errorf("replicas = %d, want 3 (from spec.values)", replicas)
			}
		case "Service":
			svc++
			if o.GetNamespace() != "podinfo-ns" {
				t.Errorf("Service ns = %s, want podinfo-ns", o.GetNamespace())
			}
		}
	}
	if depl != 1 || svc != 1 {
		t.Errorf("counts: depl=%d svc=%d", depl, svc)
	}
}

func TestRenderHelmRelease_EmptyValuesUsesDefaults(t *testing.T) {
	root := t.TempDir()
	minimalChart(t, root)

	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "x-ns"},
		Spec:       helmv2.HelmReleaseSpec{}, // no Values
	}
	objs, err := RenderHelmRelease(context.Background(), root, hr)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, o := range objs {
		if o.GetKind() != "Deployment" {
			continue
		}
		replicas, found, err := getNestedInt(o.Object, "spec", "replicas")
		if err != nil || !found {
			t.Fatalf("replicas: %v", err)
		}
		if replicas != 1 {
			t.Errorf("replicas = %d, want 1 (chart default)", replicas)
		}
	}
}

func TestRenderHelmRelease_BadChart(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Chart.yaml":                "not: valid: yaml: at: all\n",
		"templates/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\n",
	})
	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "x"},
	}
	_, err := RenderHelmRelease(context.Background(), root, hr)
	if err == nil {
		t.Fatal("expected chart load error")
	}
}

// getNestedInt is a tiny replacement for unstructured.NestedInt64
// that handles both float64 (JSON default) and int64 representations
// — Helm sometimes hands back the former depending on the
// rendering path.
func getNestedInt(obj map[string]interface{}, fields ...string) (int64, bool, error) {
	cur := interface{}(obj)
	for _, f := range fields {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return 0, false, nil
		}
		cur, ok = m[f]
		if !ok {
			return 0, false, nil
		}
	}
	switch v := cur.(type) {
	case int64:
		return v, true, nil
	case int:
		return int64(v), true, nil
	case float64:
		return int64(v), true, nil
	}
	return 0, false, nil
}
