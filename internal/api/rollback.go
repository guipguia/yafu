package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guipguia/yafu/internal/cluster"
)

// helmReconcileRequestAnnotation is the meta package's
// reconcile request annotation. helm-controller's
// ForceRequestAnnotation must equal this same value to trigger
// a force release.
const helmReconcileRequestAnnotation = "reconcile.fluxcd.io/requestedAt"

// rollback handles POST /api/v1/applications/.../rollback. It
// expects a JSON body { "revision": "<chart-version>" }.
//
// The implementation only supports HelmRelease today because
// "rollback" is only well-defined for Helm: we patch
// spec.chart.spec.version to the requested value and set the
// force-request annotation so helm-controller pulls the previous
// chart version, runs `helm upgrade`, and reconciles.
//
// Kustomization rollback is intentionally rejected — Flux always
// reconciles to the source ref's current head, so the operator
// must revert in Git. We return 422 with a clear message rather
// than fabricate a non-GitOps mutation.
func (h *applicationsHandler) rollback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Revision string `json:"revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("decode body: %v", err))
		return
	}
	if body.Revision == "" {
		writeError(w, http.StatusBadRequest, "missing required field: revision")
		return
	}

	runMutation(w, r, mutationDeps{
		registry: h.registry,
		policy:   h.policy,
		audit:    h.audit,
	}, "rollback", func(ctx context.Context, e *cluster.Entry, ns, kind, name string) error {
		switch kind {
		case "HelmRelease":
			return rollbackHelmRelease(ctx, e.Client, ns, name, body.Revision)
		case "Kustomization":
			return fmt.Errorf("kustomizations always reconcile from source HEAD; revert the change in Git instead of rolling back here")
		default:
			return fmt.Errorf("unsupported kind %q", kind)
		}
	})
}

func rollbackHelmRelease(ctx context.Context, c client.Client, ns, name, revision string) error {
	var hr helmv2.HelmRelease
	if err := c.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &hr); err != nil {
		return err
	}
	if hr.Spec.Chart == nil || hr.Spec.Chart.Spec.Version == "" {
		return fmt.Errorf("HelmRelease has no spec.chart.spec.version pinned — rollback only works for HelmReleases that pin a chart version. ChartRef-based releases must be rolled back by reverting the source revision in Git")
	}

	// Patch the chart version + set the force-reconcile token to a
	// fresh timestamp so helm-controller picks up the change on its
	// next tick rather than waiting for the spec interval.
	patch := client.MergeFrom(hr.DeepCopy())
	hr.Spec.Chart.Spec.Version = revision
	annotate(&hr, helmReconcileRequestAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
	annotate(&hr, helmv2.ForceRequestAnnotation, time.Now().UTC().Format(time.RFC3339Nano))
	return c.Patch(ctx, &hr, patch)
}
