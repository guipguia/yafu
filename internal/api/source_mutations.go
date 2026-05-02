package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/cluster"
)

// sourceMutationsHandler handles reconcile/suspend/resume on the
// FluxCD source kinds: GitRepository, HelmRepository, OCIRepository,
// Bucket. Same pipeline as application + image mutations via
// runMutation; this file just supplies the source-specific
// dispatchers.
type sourceMutationsHandler struct {
	deps mutationDeps
}

func (h *sourceMutationsHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "reconcile", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		obj, err := getSourceObj(ctx, e.Client, ns, kind, name)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotate(obj, reconcileAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
		return e.Client.Patch(ctx, obj, patch)
	})
}

func (h *sourceMutationsHandler) suspend(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "suspend", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setSourceSuspend(ctx, e.Client, ns, kind, name, true)
	})
}

func (h *sourceMutationsHandler) resume(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "resume", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setSourceSuspend(ctx, e.Client, ns, kind, name, false)
	})
}

func getSourceObj(ctx context.Context, c client.Client, ns, kind, name string) (client.Object, error) {
	var obj client.Object
	switch kind {
	case "GitRepository":
		obj = &sourcev1.GitRepository{}
	case "HelmRepository":
		obj = &sourcev1.HelmRepository{}
	case "OCIRepository":
		obj = &sourcev1.OCIRepository{}
	case "Bucket":
		obj = &sourcev1.Bucket{}
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected GitRepository, HelmRepository, OCIRepository, or Bucket)", kind)
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func setSourceSuspend(ctx context.Context, c client.Client, ns, kind, name string, suspend bool) error {
	obj, err := getSourceObj(ctx, c, ns, kind, name)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	switch v := obj.(type) {
	case *sourcev1.GitRepository:
		v.Spec.Suspend = suspend
	case *sourcev1.HelmRepository:
		v.Spec.Suspend = suspend
	case *sourcev1.OCIRepository:
		v.Spec.Suspend = suspend
	case *sourcev1.Bucket:
		v.Spec.Suspend = suspend
	default:
		return fmt.Errorf("unsupported kind")
	}
	return c.Patch(ctx, obj, patch)
}
