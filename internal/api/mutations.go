package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
)

// reconcileAnnotation is the well-known annotation Flux watches: setting it
// to a new RFC3339 timestamp triggers an immediate reconcile loop on the
// next controller tick. Same annotation is used by the `flux reconcile` CLI.
const reconcileAnnotation = "reconcile.fluxcd.io/requestedAt"

func (h *applicationsHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "reconcile", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		obj, err := getApp(ctx, e.Client, ns, kind, name)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotate(obj, reconcileAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
		return e.Client.Patch(ctx, obj, patch)
	})
}

func (h *applicationsHandler) suspend(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "suspend", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setSuspend(ctx, e.Client, ns, kind, name, true)
	})
}

func (h *applicationsHandler) resume(w http.ResponseWriter, r *http.Request) {
	h.mutate(w, r, "resume", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setSuspend(ctx, e.Client, ns, kind, name, false)
	})
}

// mutate is the shared boilerplate: extract path params, authorize,
// resolve the cluster, run the supplied action with a 10s budget, and
// translate any error into a clean status code + JSON envelope.
func (h *applicationsHandler) mutate(
	w http.ResponseWriter,
	r *http.Request,
	verb string,
	action func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error,
) {
	w.Header().Set("Content-Type", "application/json")

	clusterID := r.PathValue("cluster")
	ns := r.PathValue("ns")
	kind := r.PathValue("kind")
	name := r.PathValue("name")
	if clusterID == "" || ns == "" || kind == "" || name == "" {
		writeError(w, http.StatusBadRequest, "missing path params")
		return
	}

	id, _ := auth.IdentityFrom(r.Context())
	if !h.policy.Authorize(id, verb, clusterID) {
		writeError(w, http.StatusForbidden, fmt.Sprintf("identity is not allowed to %q on cluster %q", verb, clusterID))
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

	if err := action(ctx, e, ns, kind, name); err != nil {
		switch {
		case apierrors.IsNotFound(err):
			writeError(w, http.StatusNotFound, err.Error())
		case apierrors.IsForbidden(err):
			writeError(w, http.StatusForbidden, err.Error())
		case apierrors.IsConflict(err):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "accepted",
		"verb":    verb,
		"cluster": clusterID,
		"id":      appID(clusterID, ns, kind, name),
	})
}

func getApp(ctx context.Context, c client.Client, ns, kind, name string) (client.Object, error) {
	var obj client.Object
	switch kind {
	case "Kustomization":
		obj = &kustomizev1.Kustomization{}
	case "HelmRelease":
		obj = &helmv2.HelmRelease{}
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected Kustomization or HelmRelease)", kind)
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func setSuspend(ctx context.Context, c client.Client, ns, kind, name string, suspend bool) error {
	obj, err := getApp(ctx, c, ns, kind, name)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	switch v := obj.(type) {
	case *kustomizev1.Kustomization:
		v.Spec.Suspend = suspend
	case *helmv2.HelmRelease:
		v.Spec.Suspend = suspend
	default:
		return fmt.Errorf("unsupported kind")
	}
	return c.Patch(ctx, obj, patch)
}

func annotate(obj client.Object, key, value string) {
	annos := obj.GetAnnotations()
	if annos == nil {
		annos = map[string]string{}
	}
	annos[key] = value
	obj.SetAnnotations(annos)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
