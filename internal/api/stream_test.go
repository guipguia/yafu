package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/watch"
)

// TestStream_FiltersByPolicy proves that a user with a policy that
// only allows cluster "alpha" never sees invalidations for cluster
// "bravo", even though both clusters are publishing into the same
// hub.
func TestStream_FiltersByPolicy(t *testing.T) {
	hub := watch.NewHub()
	policy := auth.Policy{
		DefaultAction: auth.ActionDeny,
		Rules: []auth.Rule{
			{Subjects: []string{"user:alice@acme.example"}, Verbs: []string{"get"}, Clusters: []string{"alpha"}},
		},
	}
	h := &streamHandler{hub: hub, policy: policy}

	rec := newSSERecorder()
	ctx, cancel := context.WithCancel(auth.WithIdentity(context.Background(), auth.Identity{
		Subject: "uid-alice", Email: "alice@acme.example",
	}))
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()

	// Give the handler time to subscribe before publishing.
	waitForLine(t, rec, "event: hello", 200*time.Millisecond)

	hub.Publish(watch.Event{Cluster: "bravo", Kind: "Kustomization", Verb: "MODIFIED", Name: "checkout"})
	hub.Publish(watch.Event{Cluster: "alpha", Kind: "Kustomization", Verb: "MODIFIED", Name: "checkout"})

	body := readUntilSeen(t, rec, "alpha", 500*time.Millisecond)

	cancel()
	<-done

	if strings.Contains(body, `"cluster":"bravo"`) {
		t.Errorf("bravo invalidation leaked to alice; body=%q", body)
	}
	if !strings.Contains(body, `"cluster":"alpha"`) {
		t.Errorf("alpha invalidation missing for alice; body=%q", body)
	}
}

// TestStream_AllowsEveryClusterWithDefaultPolicy confirms the
// allow-all default (used in dev when --rbac-file isn't set)
// forwards every event.
func TestStream_AllowsEveryClusterWithDefaultPolicy(t *testing.T) {
	hub := watch.NewHub()
	h := &streamHandler{hub: hub, policy: auth.DefaultAllowAllPolicy}

	rec := newSSERecorder()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil).WithContext(ctx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.serve(rec, req)
	}()

	waitForLine(t, rec, "event: hello", 200*time.Millisecond)
	hub.Publish(watch.Event{Cluster: "alpha", Kind: "Kustomization", Verb: "ADDED", Name: "x"})
	hub.Publish(watch.Event{Cluster: "bravo", Kind: "HelmRelease", Verb: "ADDED", Name: "y"})

	body := readUntilSeen(t, rec, "bravo", 500*time.Millisecond)

	cancel()
	<-done

	if !strings.Contains(body, `"cluster":"alpha"`) || !strings.Contains(body, `"cluster":"bravo"`) {
		t.Errorf("expected both clusters in stream; body=%q", body)
	}
}

// --- recorder + helpers ---

// sseRecorder is httptest.ResponseRecorder + http.Flusher. The real
// recorder doesn't implement Flusher, which streamHandler.serve
// requires.
type sseRecorder struct {
	*httptest.ResponseRecorder
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (s *sseRecorder) Flush() {}

// waitForLine reads body lines until it sees one starting with prefix
// or the timeout elapses. The recorder buffer is shared with the
// running goroutine; we sleep-and-poll rather than block on read.
func waitForLine(t *testing.T, rec *sseRecorder, prefix string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := rec.Body.String()
		sc := bufio.NewScanner(strings.NewReader(body))
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), prefix) {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("did not see %q within %s; body=%q", prefix, timeout, rec.Body.String())
}

// readUntilSeen polls the recorder body until needle appears or the
// timeout elapses; returns the full body so callers can run further
// assertions on what did/didn't show up.
func readUntilSeen(t *testing.T, rec *sseRecorder, needle string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := rec.Body.String()
		if strings.Contains(body, needle) {
			return body
		}
		time.Sleep(5 * time.Millisecond)
	}
	return rec.Body.String()
}
