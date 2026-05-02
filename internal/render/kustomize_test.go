package render

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// writeFiles materialises a map of path → content under root,
// creating any intermediate directories. Used by render tests
// to spin up a tiny fake artifact tree.
func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dst, err)
		}
		if err := os.WriteFile(dst, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}
}

func TestRenderKustomization_PlainDeployment(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"kustomize/kustomization.yaml": `
resources:
  - deployment.yaml
`,
		"kustomize/deployment.yaml": `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
spec:
  replicas: 2
  selector:
    matchLabels: { app: podinfo }
  template:
    metadata: { labels: { app: podinfo } }
    spec:
      containers:
        - name: podinfo
          image: ghcr.io/stefanprodan/podinfo:6.6.2
`,
	})

	k := &kustomizev1.Kustomization{
		ObjectMeta: metav1.ObjectMeta{Name: "podinfo", Namespace: "flux-system"},
		Spec:       kustomizev1.KustomizationSpec{Path: "./kustomize"},
	}

	objs, err := RenderKustomization(context.Background(), root, k)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("got %d objects, want 1", len(objs))
	}
	if objs[0].GetKind() != "Deployment" {
		t.Errorf("kind = %s, want Deployment", objs[0].GetKind())
	}
	if objs[0].GetName() != "podinfo" {
		t.Errorf("name = %s, want podinfo", objs[0].GetName())
	}
}

func TestRenderKustomization_AppliesTargetNamespace(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"k/kustomization.yaml": `
resources:
  - svc.yaml
  - ns.yaml
`,
		"k/svc.yaml": `
apiVersion: v1
kind: Service
metadata:
  name: podinfo
spec:
  selector: { app: podinfo }
  ports: [ { port: 80 } ]
`,
		"k/ns.yaml": `
apiVersion: v1
kind: Namespace
metadata:
  name: podinfo
`,
	})

	k := &kustomizev1.Kustomization{
		Spec: kustomizev1.KustomizationSpec{
			Path:            "./k",
			TargetNamespace: "podinfo-prod",
		},
	}

	objs, err := RenderKustomization(context.Background(), root, k)
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	var svc, ns int
	for _, o := range objs {
		switch o.GetKind() {
		case "Service":
			svc++
			if o.GetNamespace() != "podinfo-prod" {
				t.Errorf("Service ns = %q, want podinfo-prod (targetNamespace)", o.GetNamespace())
			}
		case "Namespace":
			ns++
			if o.GetNamespace() != "" {
				t.Errorf("Namespace got namespace %q — must stay cluster-scoped", o.GetNamespace())
			}
		}
	}
	if svc != 1 || ns != 1 {
		t.Errorf("expected 1 Service + 1 Namespace, got %d / %d", svc, ns)
	}
}

func TestRenderKustomization_RejectsEscapingPath(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"k/kustomization.yaml": "resources: []\n",
	})
	k := &kustomizev1.Kustomization{
		Spec: kustomizev1.KustomizationSpec{Path: "../escape"},
	}
	_, err := RenderKustomization(context.Background(), root, k)
	if err == nil {
		t.Fatal("expected escape rejection")
	}
}

func TestRenderKustomization_MissingKustomization(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"empty/.gitkeep": "",
	})
	k := &kustomizev1.Kustomization{
		Spec: kustomizev1.KustomizationSpec{Path: "./empty"},
	}
	_, err := RenderKustomization(context.Background(), root, k)
	if err == nil {
		t.Fatal("expected missing-kustomization error")
	}
}

func TestRenderKustomization_DefaultsToArtifactRoot(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"kustomization.yaml": `
resources:
  - cm.yaml
`,
		"cm.yaml": `
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-cfg
data:
  key: value
`,
	})

	k := &kustomizev1.Kustomization{Spec: kustomizev1.KustomizationSpec{}} // empty path
	objs, err := RenderKustomization(context.Background(), root, k)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(objs) != 1 || objs[0].GetKind() != "ConfigMap" {
		t.Fatalf("got %v", objs)
	}
}
