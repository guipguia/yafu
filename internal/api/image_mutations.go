package api

import (
	"context"
	"fmt"
	"time"

	imageautov1 "github.com/fluxcd/image-automation-controller/api/v1"
	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"net/http"

	"github.com/guipguia/yafu/internal/cluster"
)

// imageMutationsHandler handles reconcile/suspend/resume on the
// image automation kinds: ImagePolicy, ImageRepository,
// ImageUpdateAutomation. The mutation pipeline (RBAC, audit,
// timeout, error mapping) is shared with applications via
// runMutation; this file just supplies the kind-specific
// dispatchers.
type imageMutationsHandler struct {
	deps mutationDeps
}

func (h *imageMutationsHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "reconcile", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		obj, err := getImageObj(ctx, e.Client, ns, kind, name)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotate(obj, reconcileAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
		return e.Client.Patch(ctx, obj, patch)
	})
}

func (h *imageMutationsHandler) suspend(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "suspend", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setImageSuspend(ctx, e.Client, ns, kind, name, true)
	})
}

func (h *imageMutationsHandler) resume(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "resume", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setImageSuspend(ctx, e.Client, ns, kind, name, false)
	})
}

func getImageObj(ctx context.Context, c client.Client, ns, kind, name string) (client.Object, error) {
	var obj client.Object
	switch kind {
	case "ImagePolicy":
		obj = &imagereflv1.ImagePolicy{}
	case "ImageRepository":
		obj = &imagereflv1.ImageRepository{}
	case "ImageUpdateAutomation":
		obj = &imageautov1.ImageUpdateAutomation{}
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected ImagePolicy, ImageRepository, or ImageUpdateAutomation)", kind)
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func setImageSuspend(ctx context.Context, c client.Client, ns, kind, name string, suspend bool) error {
	obj, err := getImageObj(ctx, c, ns, kind, name)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	switch v := obj.(type) {
	case *imagereflv1.ImagePolicy:
		v.Spec.Suspend = suspend
	case *imagereflv1.ImageRepository:
		v.Spec.Suspend = suspend
	case *imageautov1.ImageUpdateAutomation:
		v.Spec.Suspend = suspend
	default:
		return fmt.Errorf("unsupported kind")
	}
	return c.Patch(ctx, obj, patch)
}
