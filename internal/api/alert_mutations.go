package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/cluster"
)

// alertMutationsHandler handles reconcile/suspend/resume on the
// notification.toolkit.fluxcd.io kinds: Alert (v1beta3), Provider
// (v1beta3), Receiver (v1). Same pipeline as the other mutation
// handlers via runMutation; this file just supplies the
// notification-specific dispatchers.
type alertMutationsHandler struct {
	deps mutationDeps
}

func (h *alertMutationsHandler) reconcile(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "reconcile", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		obj, err := getAlertObj(ctx, e.Client, ns, kind, name)
		if err != nil {
			return err
		}
		patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
		annotate(obj, reconcileAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
		return e.Client.Patch(ctx, obj, patch)
	})
}

func (h *alertMutationsHandler) suspend(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "suspend", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setAlertSuspend(ctx, e.Client, ns, kind, name, true)
	})
}

func (h *alertMutationsHandler) resume(w http.ResponseWriter, r *http.Request) {
	runMutation(w, r, h.deps, "resume", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		return setAlertSuspend(ctx, e.Client, ns, kind, name, false)
	})
}

func getAlertObj(ctx context.Context, c client.Client, ns, kind, name string) (client.Object, error) {
	var obj client.Object
	switch kind {
	case "Alert":
		obj = &notificationv1beta3.Alert{}
	case "Provider":
		obj = &notificationv1beta3.Provider{}
	case "Receiver":
		obj = &notificationv1.Receiver{}
	default:
		return nil, fmt.Errorf("unsupported kind %q (expected Alert, Provider, or Receiver)", kind)
	}
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func setAlertSuspend(ctx context.Context, c client.Client, ns, kind, name string, suspend bool) error {
	obj, err := getAlertObj(ctx, c, ns, kind, name)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(obj.DeepCopyObject().(client.Object))
	switch v := obj.(type) {
	case *notificationv1beta3.Alert:
		v.Spec.Suspend = suspend
	case *notificationv1beta3.Provider:
		v.Spec.Suspend = suspend
	case *notificationv1.Receiver:
		v.Spec.Suspend = suspend
	default:
		return fmt.Errorf("unsupported kind")
	}
	return c.Patch(ctx, obj, patch)
}
