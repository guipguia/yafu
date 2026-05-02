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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apitypes "github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
)

// fluxManagers is the set of metadata.managedFields.Manager strings that
// indicate Flux itself is the owner. Anything else (kubectl, k9s,
// kubectl-edit, terraform, helm-cli, …) is treated as drift.
var fluxManagers = map[string]bool{
	"kustomize-controller":     true,
	"helm-controller":          true,
	"source-controller":        true,
	"notification-controller":  true,
	"image-reflector-controller": true,
	"image-automation-controller": true,
}

func (h *applicationsHandler) diff(w http.ResponseWriter, r *http.Request) {
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

	resp := apitypes.DiffResponse{
		AppID:     appID(clusterID, ns, kind, name),
		Resources: []apitypes.DriftedResource{},
		Note:      "v0.1 surfaces field-ownership drift via metadata.managedFields. True Git-vs-cluster diff (kustomize build / helm render against the source) lands in v0.4.",
	}

	entries, _ := inventoryEntriesOf(obj)
	if len(entries) == 0 {
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	resp.Resources = checkDrift(ctx, e.Client, entries)
	sort.Slice(resp.Resources, func(i, j int) bool {
		a, b := resp.Resources[i], resp.Resources[j]
		// Drifted entries first.
		if a.Status != b.Status {
			return a.Status == "drift" && b.Status != "drift"
		}
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

func checkDrift(ctx context.Context, c client.Client, refs []inventoryRef) []apitypes.DriftedResource {
	out := make([]apitypes.DriftedResource, len(refs))
	sem := make(chan struct{}, maxConcurrentInventoryFetches)
	var wg sync.WaitGroup
	for i, ref := range refs {
		i, ref := i, ref
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			out[i] = checkOneDrift(ctx, c, ref)
		}()
	}
	wg.Wait()
	return out
}

func checkOneDrift(ctx context.Context, c client.Client, ref inventoryRef) apitypes.DriftedResource {
	d := apitypes.DriftedResource{
		Group: ref.group, Version: ref.version, Kind: ref.kind,
		Ns: ref.ns, Name: ref.name, Status: "unknown",
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: ref.group, Version: ref.version, Kind: ref.kind})
	if err := c.Get(ctx, types.NamespacedName{Namespace: ref.ns, Name: ref.name}, obj); err != nil {
		if apierrors.IsNotFound(err) {
			d.Status = "notfound"
			return d
		}
		return d
	}

	d.Managers, d.Status = classifyManagers(obj.GetManagedFields())
	return d
}

// classifyManagers turns a slice of ManagedFieldsEntry into the DTO form
// and returns "drift" when any non-Flux manager has written, else "ready".
// Extracted so the rule is unit-testable without standing up a fake k8s
// client (which has strict validation on managedFields entries).
func classifyManagers(mfs []metav1.ManagedFieldsEntry) ([]apitypes.ManagedField, string) {
	if len(mfs) == 0 {
		return nil, "ready"
	}
	out := make([]apitypes.ManagedField, 0, len(mfs))
	hasForeign := false
	for _, mf := range mfs {
		mgr := strings.TrimSuffix(mf.Manager, ".exe")
		foreign := !fluxManagers[mgr]
		entry := apitypes.ManagedField{
			Manager:   mgr,
			Operation: string(mf.Operation),
			Foreign:   foreign,
		}
		if mf.Time != nil {
			entry.Time = mf.Time.UTC().Format(time.RFC3339)
		}
		out = append(out, entry)
		if foreign {
			hasForeign = true
		}
	}
	if hasForeign {
		return out, "drift"
	}
	return out, "ready"
}
