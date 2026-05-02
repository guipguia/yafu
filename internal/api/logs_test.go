package api

import (
	"bytes"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/guipguia/yafu/internal/api/types"
)

func TestUniqueInventoryNamespaces(t *testing.T) {
	got := uniqueInventoryNamespaces([]inventoryRef{
		{ns: "shop"},
		{ns: ""}, // cluster-scoped, skipped
		{ns: "shop"},
		{ns: "ml"},
	})
	if len(got) != 2 || got[0] != "ml" || got[1] != "shop" {
		t.Errorf("got %v, want [ml shop] (sorted, unique, no empty)", got)
	}
}

func TestPodMatchesInventory(t *testing.T) {
	entries := []inventoryRef{
		{ns: "shop", kind: "Deployment", name: "checkout-api"},
		{ns: "shop", kind: "Service", name: "checkout-api"}, // not a workload, not relevant
		{ns: "ml", kind: "StatefulSet", name: "ranker"},
	}

	cases := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			"deployment-managed pod in shop",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "checkout-api-7d9c4b-x4k2", Namespace: "shop"}},
			true,
		},
		{
			"unrelated pod in shop",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "kube-proxy-abcd", Namespace: "shop"}},
			false,
		},
		{
			"matching prefix but wrong namespace",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "checkout-api-7d9c4b-x4k2", Namespace: "data"}},
			false,
		},
		{
			"statefulset-managed pod in ml",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ranker-0", Namespace: "ml"}},
			true,
		},
		{
			"exact match (raw Pod kind)",
			&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "ranker", Namespace: "ml"}},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := podMatchesInventory(c.pod, c.pod.Namespace, entries); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestPodMatchesInventory_NoEntries(t *testing.T) {
	// Empty inventory should accept anything (HelmRelease v0.1 fallback path).
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "any", Namespace: "default"}}
	if !podMatchesInventory(pod, "default", nil) {
		t.Error("empty inventory should accept any pod")
	}
}

func TestPodToInfo(t *testing.T) {
	now := time.Now()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "checkout-api-7d9c4b-x4k2",
			Namespace:         "shop",
			CreationTimestamp: metav1.NewTime(now.Add(-30 * time.Minute)),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app"},
				{Name: "sidecar"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "app", RestartCount: 14},
				{Name: "sidecar", RestartCount: 0},
			},
		},
	}
	got := podToInfo(pod, now)
	if got.Phase != "Running" || got.Restarts != 14 {
		t.Errorf("phase/restarts wrong: %+v", got)
	}
	if len(got.Containers) != 2 || got.Containers[0] != "app" {
		t.Errorf("containers = %v", got.Containers)
	}
	if got.Age != "30m" {
		t.Errorf("age = %q, want 30m", got.Age)
	}
}

func TestPickPod(t *testing.T) {
	pods := []types.PodInfo{
		{Ns: "shop", Name: "a", Phase: "Pending"},
		{Ns: "shop", Name: "b", Phase: "Running"},
		{Ns: "shop", Name: "c", Phase: "Running"},
	}

	if got := pickPod(pods, ""); got == nil || got.Name != "b" {
		t.Errorf("default should pick first Running pod, got %+v", got)
	}
	if got := pickPod(pods, "shop/c"); got == nil || got.Name != "c" {
		t.Errorf("ns/name lookup failed: %+v", got)
	}
	if got := pickPod(pods, "c"); got == nil || got.Name != "c" {
		t.Errorf("bare name lookup failed: %+v", got)
	}
	if got := pickPod(pods, "missing"); got != nil {
		t.Errorf("non-existent pod should return nil, got %+v", got)
	}
	if got := pickPod(nil, ""); got != nil {
		t.Errorf("empty list → nil")
	}
	pendingOnly := []types.PodInfo{{Ns: "x", Name: "p", Phase: "Pending"}}
	if got := pickPod(pendingOnly, ""); got == nil || got.Name != "p" {
		t.Errorf("no Running pods → first available, got %+v", got)
	}
}

func TestParseTail(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"", defaultLogTail},
		{"50", 50},
		{"-3", defaultLogTail},
		{"abc", defaultLogTail},
		{"99999", maxLogTail},
	}
	for _, c := range cases {
		if got := parseTail(c.in); got != c.want {
			t.Errorf("parseTail(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestWriteSSE(t *testing.T) {
	cases := []struct {
		name      string
		event     string
		data      string
		wantBytes string
	}{
		{
			name:      "unnamed message single line",
			event:     "",
			data:      "hello",
			wantBytes: "data: hello\n\n",
		},
		{
			name:      "named event",
			event:     "open",
			data:      "ready",
			wantBytes: "event: open\ndata: ready\n\n",
		},
		{
			name:      "multiline data emits one data: per line",
			event:     "",
			data:      "line one\nline two\nline three",
			wantBytes: "data: line one\ndata: line two\ndata: line three\n\n",
		},
		{
			name:      "error event",
			event:     "error",
			data:      "boom",
			wantBytes: "event: error\ndata: boom\n\n",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			writeSSE(&buf, c.event, c.data)
			if got := buf.String(); got != c.wantBytes {
				t.Errorf("got %q\nwant %q", got, c.wantBytes)
			}
		})
	}
}

func TestContainerSuffix(t *testing.T) {
	if got := containerSuffix(""); got != "" {
		t.Errorf("empty container should produce no suffix, got %q", got)
	}
	if got := containerSuffix("app"); got != " container=app" {
		t.Errorf("got %q, want \" container=app\"", got)
	}
}

func TestHumanizeAgeFrom(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		offset time.Duration
		want   string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{3 * 24 * time.Hour, "3d"},
	}
	for _, c := range cases {
		got := humanizeAgeFrom(now.Add(-c.offset), now)
		if got != c.want {
			t.Errorf("offset=%v got %q, want %q", c.offset, got, c.want)
		}
	}
}
