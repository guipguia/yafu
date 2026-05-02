package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apitypes "github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

// maxConcurrentInventoryFetches caps the parallelism per-app so a
// HelmRelease with hundreds of resources doesn't open hundreds of
// concurrent connections to the remote API server.
const maxConcurrentInventoryFetches = 16

func (h *applicationsHandler) tree(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	clusterID := r.PathValue("cluster")
	ns := r.PathValue("ns")
	kind := r.PathValue("kind")
	name := r.PathValue("name")

	id, _ := auth.IdentityFrom(r.Context())
	if !h.policy.Authorize(id, "get", clusterID) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("identity is not allowed to get cluster %q", clusterID))
		return
	}

	if h.registry == nil {
		writeError(w, http.StatusServiceUnavailable, "registry not initialised")
		return
	}
	e, ok := h.registry.Get(clusterID)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("cluster %q not registered", clusterID))
		return
	}
	if !e.Status().Reachable {
		writeError(w, http.StatusServiceUnavailable, "cluster unreachable")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	obj, err := getApp(ctx, e.Client, ns, kind, name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := apitypes.TreeResponse{
		AppID: appID(clusterID, ns, kind, name),
		Nodes: []apitypes.TreeNode{},
	}

	entries, note := inventoryEntriesOf(obj)
	resp.Note = note
	if len(entries) == 0 {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Nodes = fetchInventory(ctx, e, entries)
	sort.Slice(resp.Nodes, func(i, j int) bool {
		a, b := resp.Nodes[i], resp.Nodes[j]
		if a.Ns != b.Ns {
			return a.Ns < b.Ns
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})

	_ = json.NewEncoder(w).Encode(resp)
}

// inventoryRef is the parsed form of one Flux ResourceRef (same shape for
// Kustomization and HelmRelease).
type inventoryRef struct {
	ns, name, group, kind, version string
}

// inventoryEntriesOf returns the inventory of a Kustomization or
// HelmRelease as parsed refs. The returned note is non-empty when the
// inventory hasn't been populated yet, or when the kind doesn't expose
// a parseable inventory at all (HelmRelease today — see v0.3 note).
func inventoryEntriesOf(obj any) ([]inventoryRef, string) {
	switch v := obj.(type) {
	case *kustomizev1.Kustomization:
		if v.Status.Inventory == nil || len(v.Status.Inventory.Entries) == 0 {
			return nil, "no inventory recorded — Kustomization may not have reconciled yet"
		}
		out := make([]inventoryRef, 0, len(v.Status.Inventory.Entries))
		for _, ref := range v.Status.Inventory.Entries {
			if r, ok := parseInventoryID(ref.ID, ref.Version); ok {
				out = append(out, r)
			}
		}
		return out, ""

	case *helmv2.HelmRelease:
		// helm-controller v2 stores per-snapshot inventories in
		// Helm's own storage Secret, not on the HelmRelease status.
		// Walking that lands in v0.3.
		_ = v
		return nil, "HelmRelease resource tree lands in v0.3 — requires reading Helm's release storage"
	}
	return nil, ""
}

// parseInventoryID splits Flux's "<ns>_<name>_<group>_<kind>" encoding.
// Cluster-scoped resources have empty ns; core API resources have empty
// group.
func parseInventoryID(id, version string) (inventoryRef, bool) {
	parts := strings.SplitN(id, "_", 4)
	if len(parts) != 4 {
		return inventoryRef{}, false
	}
	return inventoryRef{
		ns:      parts[0],
		name:    parts[1],
		group:   parts[2],
		kind:    parts[3],
		version: version,
	}, true
}

func fetchInventory(ctx context.Context, e *cluster.Entry, refs []inventoryRef) []apitypes.TreeNode {
	out := make([]apitypes.TreeNode, len(refs))
	sem := make(chan struct{}, maxConcurrentInventoryFetches)
	var wg sync.WaitGroup
	for i, ref := range refs {
		i, ref := i, ref
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = fetchOneResource(ctx, e.Client, ref)
		}()
	}
	wg.Wait()
	return out
}

func fetchOneResource(ctx context.Context, c client.Client, ref inventoryRef) apitypes.TreeNode {
	node := apitypes.TreeNode{
		Group:   ref.group,
		Version: ref.version,
		Kind:    ref.kind,
		Ns:      ref.ns,
		Name:    ref.name,
		Status:  "unknown",
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   ref.group,
		Version: ref.version,
		Kind:    ref.kind,
	})
	if err := c.Get(ctx, types.NamespacedName{Namespace: ref.ns, Name: ref.name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			node.Status = "notfound"
			node.Message = "resource not present on cluster"
			return node
		}
		node.Status = "unknown"
		node.Message = err.Error()
		return node
	}

	node.Status, node.Message = unstructuredStatus(obj)
	return node
}

// unstructuredStatus returns a coarse status + a one-line message for any
// k8s object. Strategy in priority order:
//   1. Ready condition under status.conditions  → ready/failing
//   2. Deployment-style status.readyReplicas vs status.replicas → ready/progressing
//   3. fallback "ready" — the resource exists, that's all we can say
func unstructuredStatus(obj *unstructured.Unstructured) (status, message string) {
	conds, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, c := range conds {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := cm["type"].(string); t != "Ready" {
				continue
			}
			st, _ := cm["status"].(string)
			msg, _ := cm["message"].(string)
			if st == string(metav1.ConditionTrue) {
				return "ready", msg
			}
			return "failing", msg
		}
	}

	if ready, ok, _ := unstructured.NestedInt64(obj.Object, "status", "readyReplicas"); ok {
		desired, ok2, _ := unstructured.NestedInt64(obj.Object, "status", "replicas")
		if !ok2 {
			desired, _, _ = unstructured.NestedInt64(obj.Object, "spec", "replicas")
		}
		if desired == 0 || ready >= desired {
			return "ready", fmt.Sprintf("%d/%d ready", ready, desired)
		}
		return "progressing", fmt.Sprintf("%d/%d ready", ready, desired)
	}

	return "ready", ""
}
