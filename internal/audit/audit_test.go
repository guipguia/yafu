package audit

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogger_RecordWritesOneJSONLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	l.Record(Record{
		RequestID: "req-1",
		Identity:  Identity{Subject: "maria", Email: "m@example.com", Groups: []string{"admins"}},
		Verb:      "reconcile",
		Resource:  Resource{Cluster: "prod", Ns: "shop", Kind: "Kustomization", Name: "checkout"},
		Outcome:   OutcomeOK,
	})

	got := strings.TrimSpace(buf.String())
	if !strings.HasSuffix(got, "}") || strings.Contains(got, "\n") {
		t.Errorf("expected exactly one line of JSON, got %q", got)
	}

	var r Record
	if err := json.Unmarshal([]byte(got), &r); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if r.Verb != "reconcile" || r.Outcome != OutcomeOK {
		t.Errorf("decoded record wrong: %+v", r)
	}
	if r.Timestamp.IsZero() {
		t.Error("timestamp should be filled when zero")
	}
	if r.Identity.Subject != "maria" || len(r.Identity.Groups) != 1 {
		t.Errorf("identity round-trip wrong: %+v", r.Identity)
	}
}

func TestLogger_PreservesProvidedTimestamp(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	fixed := time.Date(2026, 5, 2, 15, 0, 0, 0, time.UTC)
	l.Record(Record{Timestamp: fixed, Verb: "x", Outcome: OutcomeOK})

	var r Record
	if err := json.Unmarshal(buf.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if !r.Timestamp.Equal(fixed) {
		t.Errorf("timestamp = %v, want %v", r.Timestamp, fixed)
	}
}

func TestLogger_ConcurrentSafe(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.Record(Record{Verb: "concurrent", Outcome: OutcomeOK})
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != n {
		t.Fatalf("expected %d lines, got %d", n, len(lines))
	}
	// Every line must be valid JSON — proves no interleaving.
	for i, line := range lines {
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Errorf("line %d is malformed: %v\n%s", i, err, line)
		}
	}
}

func TestDiscard(t *testing.T) {
	l := Discard()
	// Just must not panic.
	l.Record(Record{Verb: "x", Outcome: OutcomeOK})
}
