package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const sampleManifest = `# Source: chart/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: monitoring
  namespace: observability
spec:
  replicas: 1
---
# Source: chart/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: monitoring
  namespace: observability
---
# Source: chart/templates/test-hook.yaml
apiVersion: v1
kind: Pod
metadata:
  name: monitoring-test
  namespace: observability
  annotations:
    helm.sh/hook: test
spec:
  containers:
    - name: test
      image: busybox
---
# Source: chart/templates/clusterrole.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: monitoring-reader
rules: []
`

func TestParseManifestRefs(t *testing.T) {
	refs := parseManifestRefs(sampleManifest, "observability")
	// Hook should be skipped → 3 refs (Deployment, Service, ClusterRole).
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3 (hook skipped); refs=%+v", len(refs), refs)
	}

	by := map[string]inventoryRef{}
	for _, r := range refs {
		by[r.kind+"/"+r.name] = r
	}

	if got, ok := by["Deployment/monitoring"]; !ok || got.group != "apps" || got.version != "v1" || got.ns != "observability" {
		t.Errorf("Deployment ref wrong: %+v", got)
	}
	if got, ok := by["Service/monitoring"]; !ok || got.group != "" || got.version != "v1" {
		t.Errorf("Service ref wrong: %+v", got)
	}
	if got, ok := by["ClusterRole/monitoring-reader"]; !ok {
		t.Errorf("ClusterRole missing")
	} else if got.ns != "observability" {
		t.Errorf("cluster-scoped resource should still take defaultNS for grouping, got ns=%q", got.ns)
	}

	if _, hookLeaked := by["Pod/monitoring-test"]; hookLeaked {
		t.Error("Helm hook leaked into inventory; should be filtered")
	}
}

func TestParseManifestRefs_Empty(t *testing.T) {
	if got := parseManifestRefs("", "default"); len(got) != 0 {
		t.Errorf("empty manifest should yield no refs, got %+v", got)
	}
}

func TestDecodeHelmReleaseSecret_RoundTrip(t *testing.T) {
	rel := helmReleaseStorage{
		Name:      "monitoring",
		Namespace: "observability",
		Version:   3,
		Manifest:  sampleManifest,
	}
	encoded := encodeHelmRelease(t, rel)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.monitoring.v3",
			Namespace: "observability",
			Labels:    map[string]string{"owner": "helm"},
		},
		Data: map[string][]byte{
			"release": []byte(encoded),
		},
	}

	got, err := decodeHelmReleaseSecret(secret)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "monitoring" || got.Version != 3 {
		t.Errorf("identity wrong: %+v", got)
	}
	if got.Manifest != sampleManifest {
		t.Errorf("manifest round-trip mismatched")
	}
}

func TestDecodeHelmReleaseSecret_Uncompressed(t *testing.T) {
	// Pre-2.0.0 Helm releases didn't gzip; verify the fallback path.
	rel := helmReleaseStorage{Name: "old", Namespace: "default", Version: 1, Manifest: "kind: ConfigMap"}
	jsonBytes, _ := json.Marshal(rel)
	encoded := base64.StdEncoding.EncodeToString(jsonBytes)

	secret := &corev1.Secret{Data: map[string][]byte{"release": []byte(encoded)}}
	got, err := decodeHelmReleaseSecret(secret)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "old" {
		t.Errorf("name = %q", got.Name)
	}
}

func TestDecodeHelmReleaseSecret_MissingKey(t *testing.T) {
	secret := &corev1.Secret{Data: map[string][]byte{}}
	if _, err := decodeHelmReleaseSecret(secret); err == nil {
		t.Error("expected error for missing 'release' key")
	}
}

func TestHelmReleaseInventory_NoHistoryReturnsNote(t *testing.T) {
	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "monitoring", Namespace: "observability"},
		// No status.history → not yet installed.
	}
	refs, note, err := helmReleaseInventory(context.Background(), fake.NewSimpleClientset(), hr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(refs) != 0 {
		t.Errorf("expected no refs, got %+v", refs)
	}
	if note == "" {
		t.Error("expected explanatory note for missing release history")
	}
}

func TestHelmReleaseInventory_LoadsFromSecret(t *testing.T) {
	rel := helmReleaseStorage{
		Name:      "monitoring",
		Namespace: "observability",
		Version:   3,
		Manifest:  sampleManifest,
	}
	encoded := encodeHelmRelease(t, rel)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sh.helm.release.v1.monitoring.v3",
			Namespace: "observability",
			Labels:    map[string]string{"owner": "helm"},
		},
		Data: map[string][]byte{"release": []byte(encoded)},
	}
	kube := fake.NewSimpleClientset(secret)

	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "monitoring", Namespace: "observability"},
		Status: helmv2.HelmReleaseStatus{
			StorageNamespace: "observability",
			History: helmv2.Snapshots{
				{Name: "monitoring", Namespace: "observability", Version: 3, Status: "deployed"},
			},
		},
	}

	refs, note, err := helmReleaseInventory(context.Background(), kube, hr)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if note != "" {
		t.Errorf("unexpected note: %q", note)
	}
	if len(refs) != 3 {
		t.Fatalf("got %d refs, want 3 (Deployment, Service, ClusterRole)", len(refs))
	}
}

func TestHelmReleaseInventory_SecretNotFoundReturnsNote(t *testing.T) {
	hr := &helmv2.HelmRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "monitoring", Namespace: "observability"},
		Status: helmv2.HelmReleaseStatus{
			History: helmv2.Snapshots{{Name: "monitoring", Namespace: "observability", Version: 3}},
		},
	}
	refs, note, err := helmReleaseInventory(context.Background(), fake.NewSimpleClientset(), hr)
	if err != nil {
		t.Fatalf("expected no error for missing secret, got %v", err)
	}
	if len(refs) != 0 {
		t.Error("expected empty refs when secret not found")
	}
	if note == "" {
		t.Error("expected note pointing at missing storage Secret")
	}
}

func encodeHelmRelease(t *testing.T, rel helmReleaseStorage) string {
	t.Helper()
	jsonBytes, err := json.Marshal(rel)
	if err != nil {
		t.Fatal(err)
	}
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	if _, err := gz.Write(jsonBytes); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(gzBuf.Bytes())
}
