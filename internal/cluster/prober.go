package cluster

import (
	"context"
	"fmt"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Probe checks reachability, detects Flux, and returns a fresh status
// snapshot for the entry. It never returns an error — failures are
// recorded in Status.LastError so the caller can persist a partial state.
func Probe(ctx context.Context, e *Entry) Status {
	s := Status{LastProbe: time.Now()}

	v, err := e.Discovery.ServerVersion()
	if err != nil {
		s.LastError = fmt.Sprintf("discovery: %v", err)
		return s
	}
	s.Reachable = true
	s.KubernetesVersion = v.GitVersion

	// Detect Flux by listing Kustomizations cluster-wide. The CRD being
	// absent shows up as NoMatch / NotFound from a real cluster, or as a
	// scheme registration error from controller-runtime's fake client in
	// tests — treat all three as "Flux not installed", not a probe error.
	var ks kustomizev1.KustomizationList
	if err := e.Client.List(ctx, &ks); err != nil {
		if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) || runtime.IsNotRegisteredError(err) {
			return s
		}
		s.LastError = fmt.Sprintf("list kustomizations: %v", err)
		return s
	}
	s.FluxInstalled = true

	for i := range ks.Items {
		k := &ks.Items[i]
		s.Summary.Apps++
		switch {
		case k.Spec.Suspend:
			s.Summary.Suspended++
		case isReady(k.Status.Conditions):
			s.Summary.Ready++
		default:
			s.Summary.Failing++
		}
	}

	var hr helmv2.HelmReleaseList
	if err := e.Client.List(ctx, &hr); err == nil {
		for i := range hr.Items {
			h := &hr.Items[i]
			s.Summary.Apps++
			switch {
			case h.Spec.Suspend:
				s.Summary.Suspended++
			case isReady(h.Status.Conditions):
				s.Summary.Ready++
			default:
				s.Summary.Failing++
			}
		}
	}

	var gits sourcev1.GitRepositoryList
	if err := e.Client.List(ctx, &gits); err == nil {
		s.Summary.Sources += len(gits.Items)
	}
	var helms sourcev1.HelmRepositoryList
	if err := e.Client.List(ctx, &helms); err == nil {
		s.Summary.Sources += len(helms.Items)
	}
	var ocis sourcev1.OCIRepositoryList
	if err := e.Client.List(ctx, &ocis); err == nil {
		s.Summary.Sources += len(ocis.Items)
	}
	var buckets sourcev1.BucketList
	if err := e.Client.List(ctx, &buckets); err == nil {
		s.Summary.Sources += len(buckets.Items)
	}

	return s
}

func isReady(conds []metav1.Condition) bool {
	for i := range conds {
		if conds[i].Type == "Ready" {
			return conds[i].Status == metav1.ConditionTrue
		}
	}
	return false
}
