package cluster

import (
	"fmt"
	"sync"
	"testing"
)

func TestCRDRegistry_BasicCRUD(t *testing.T) {
	r := NewCRDRegistry()

	if got := r.List(); len(got) != 0 {
		t.Fatalf("expected empty registry, got %d", len(got))
	}
	if _, ok := r.Get("missing"); ok {
		t.Fatal("expected Get to return false for missing entry")
	}

	a := &Entry{Name: "a"}
	b := &Entry{Name: "b"}
	r.Set(a.Name, a)
	r.Set(b.Name, b)

	if got, ok := r.Get("a"); !ok || got != a {
		t.Errorf("Get(a) = %v, %v; want %v, true", got, ok, a)
	}
	if got := r.List(); len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("List = %+v, want insertion-order [a, b]", got)
	}

	// Replace 'a' — should not duplicate in order.
	a2 := &Entry{Name: "a"}
	r.Set(a.Name, a2)
	got := r.List()
	if len(got) != 2 {
		t.Errorf("List after replace = %d entries, want 2", len(got))
	}
	if got[0] != a2 {
		t.Errorf("expected replaced entry pointer, got %p", got[0])
	}

	r.Delete("a")
	if _, ok := r.Get("a"); ok {
		t.Error("expected Get to return false after Delete")
	}
	if got := r.List(); len(got) != 1 || got[0].Name != "b" {
		t.Errorf("after delete a, List = %+v, want [b]", got)
	}
}

func TestCRDRegistry_Concurrent(t *testing.T) {
	// Smoke test under -race; correctness is asserted by the race detector
	// catching any unsynchronised access.
	r := NewCRDRegistry()
	const n = 100
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("c%03d", i)
			r.Set(name, &Entry{Name: name})
			if _, ok := r.Get(name); !ok {
				t.Errorf("just-set entry %q missing on Get", name)
			}
			_ = r.List()
			if i%2 == 0 {
				r.Delete(name)
			}
		}(i)
	}
	wg.Wait()

	got := r.List()
	if len(got) != n/2 {
		t.Errorf("after concurrent churn: List has %d entries, want %d", len(got), n/2)
	}
}
