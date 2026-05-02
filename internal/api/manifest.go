package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	apitypes "github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
)

// noiseFields are removed from the unstructured object before YAML
// serialization. These are server-managed fields whose presence in the
// rendered YAML would obscure the meaningful spec/status diff.
var noiseFields = [][]string{
	{"metadata", "managedFields"},
	{"metadata", "resourceVersion"},
	{"metadata", "uid"},
	{"metadata", "selfLink"},
	{"metadata", "creationTimestamp"},
	{"metadata", "generation"},
}

func (h *applicationsHandler) manifest(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
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

	yamlText, err := manifestYAML(obj, kind)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = json.NewEncoder(w).Encode(apitypes.ManifestResponse{
		AppID: appID(clusterID, ns, kind, name),
		Kind:  kind,
		YAML:  yamlText,
	})
}

// manifestYAML converts a typed Flux resource to clean YAML by routing it
// through Unstructured, stripping server-managed fields, then serializing.
// expectedKind is used to rehydrate apiVersion/kind when the typed Get
// returned an empty TypeMeta (controller-runtime does not always
// populate it from the scheme).
func manifestYAML(obj any, expectedKind string) (string, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return "", fmt.Errorf("convert to unstructured: %w", err)
	}
	if av, _ := u["apiVersion"].(string); av == "" {
		u["apiVersion"] = apiVersionForKind(expectedKind)
	}
	if k, _ := u["kind"].(string); k == "" {
		u["kind"] = expectedKind
	}
	for _, path := range noiseFields {
		unstructured.RemoveNestedField(u, path...)
	}
	bytes, err := yaml.Marshal(u)
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	return string(bytes), nil
}

func apiVersionForKind(kind string) string {
	switch kind {
	case "Kustomization":
		return "kustomize.toolkit.fluxcd.io/v1"
	case "HelmRelease":
		return "helm.toolkit.fluxcd.io/v2"
	}
	return ""
}
