package api

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// sseRecorder is a thread-safe ResponseWriter for SSE tests. The
// stock httptest.ResponseRecorder isn't safe for the pattern these
// tests use — handler goroutine writes to the body while the test
// goroutine polls it for assertion strings. We protect the buffer
// with a mutex and override Body() to return a fresh String snapshot.
//
// Also implements http.Flusher because streamHandler.serve requires
// it on the response writer.
type sseRecorder struct {
	mu      sync.Mutex
	headers http.Header
	buf     bytes.Buffer
	status  int
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{
		headers: http.Header{},
		status:  http.StatusOK,
	}
}

func (s *sseRecorder) Header() http.Header { return s.headers }

func (s *sseRecorder) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *sseRecorder) WriteHeader(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = code
}

func (s *sseRecorder) Flush() {}

// Body returns the current accumulated body as a string. Safe to
// call from any goroutine.
func (s *sseRecorder) Body() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// waitForLine polls the recorder body until a line starting with
// prefix appears or the timeout elapses. Sleep-and-poll because we
// don't have a way to block on a specific output from the handler
// goroutine without buffering everything through a channel.
func waitForLine(t *testing.T, rec *sseRecorder, prefix string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := rec.Body()
		sc := bufio.NewScanner(strings.NewReader(body))
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), prefix) {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("did not see %q within %s; body=%q", prefix, timeout, rec.Body())
}

// readUntilSeen polls the recorder body until needle appears or the
// timeout elapses; returns the full body so callers can run further
// assertions on what did/didn't show up.
func readUntilSeen(t *testing.T, rec *sseRecorder, needle string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		body := rec.Body()
		if strings.Contains(body, needle) {
			return body
		}
		time.Sleep(5 * time.Millisecond)
	}
	return rec.Body()
}
