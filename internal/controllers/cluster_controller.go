package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	yafuv1alpha1 "github.com/guipguia/yafu/api/v1alpha1"
	"github.com/guipguia/yafu/internal/cluster"
)

// ClusterReconciler reconciles yafu.io Cluster resources, building a per-cluster
// client and probing reachability + Flux installation, then publishing the
// Entry to the in-memory CRDRegistry the HTTP API reads from.
type ClusterReconciler struct {
	client.Client
	Registry     *cluster.CRDRegistry
	HomeConfig   *rest.Config
	OwnNamespace string
	ProbeEvery   time.Duration
}

// SetupWithManager wires the controller to the manager.
func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ProbeEvery == 0 {
		r.ProbeEvery = 30 * time.Second
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&yafuv1alpha1.Cluster{}).
		Complete(r)
}

// Reconcile is the controller entrypoint. It is intentionally idempotent and
// requeues on its own cadence to refresh probe state even when the spec
// hasn't changed.
func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cr yafuv1alpha1.Cluster
	if err := r.Get(ctx, req.NamespacedName, &cr); err != nil {
		if apierrors.IsNotFound(err) {
			r.Registry.Delete(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cfg, credErr := r.resolveConnection(ctx, cr.Spec.Connection)
	if credErr != nil {
		logger.Error(credErr, "resolve connection")
		r.Registry.Delete(cr.Name)
		r.setReachable(&cr, false, "CredentialsError", credErr.Error())
		r.setReady(&cr, false, "CredentialsError", credErr.Error())
		return r.requeueAfter(ctx, &cr)
	}

	cl, disco, err := cluster.NewClients(cfg)
	if err != nil {
		logger.Error(err, "build clients")
		r.Registry.Delete(cr.Name)
		r.setReachable(&cr, false, "ClientBuildError", err.Error())
		r.setReady(&cr, false, "ClientBuildError", err.Error())
		return r.requeueAfter(ctx, &cr)
	}

	e := &cluster.Entry{
		Name:          cr.Name,
		DisplayName:   defaultIfEmpty(cr.Spec.DisplayName, cr.Name),
		Region:        cr.Spec.Region,
		Environment:   string(cr.Spec.Environment),
		FluxNamespace: defaultIfEmpty(cr.Spec.FluxNamespace, "flux-system"),
		Client:        cl,
		Discovery:     disco,
	}

	probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	st := cluster.Probe(probeCtx, e)
	e.SetStatus(st)
	r.Registry.Set(cr.Name, e)

	cr.Status.ObservedGeneration = cr.Generation
	cr.Status.KubernetesVersion = st.KubernetesVersion
	cr.Status.FluxVersion = st.FluxVersion
	cr.Status.LastProbeTime = &metav1.Time{Time: st.LastProbe}
	cr.Status.Summary = &yafuv1alpha1.ClusterSummary{
		Apps:      int32(st.Summary.Apps),
		Ready:     int32(st.Summary.Ready),
		Failing:   int32(st.Summary.Failing),
		Suspended: int32(st.Summary.Suspended),
		Sources:   int32(st.Summary.Sources),
	}
	r.setReachable(&cr, st.Reachable, reachableReason(st), reachableMessage(st))
	r.setFluxInstalled(&cr, st.FluxInstalled, fluxReason(st), fluxMessage(st))
	r.setReady(&cr, st.Reachable && st.LastError == "", readyReason(st), readyMessage(st))

	return r.requeueAfter(ctx, &cr)
}

func (r *ClusterReconciler) requeueAfter(ctx context.Context, cr *yafuv1alpha1.Cluster) (ctrl.Result, error) {
	if err := r.Status().Update(ctx, cr); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.ProbeEvery}, nil
}

func (r *ClusterReconciler) resolveConnection(ctx context.Context, conn yafuv1alpha1.ClusterConnection) (*rest.Config, error) {
	switch {
	case conn.InCluster:
		if r.HomeConfig == nil {
			return nil, fmt.Errorf("inCluster requested but home config unavailable")
		}
		// Return a copy so callers can mutate without affecting the manager's config.
		c := rest.CopyConfig(r.HomeConfig)
		return c, nil
	case conn.KubeconfigSecretRef != nil:
		ns := conn.KubeconfigSecretRef.Namespace
		if ns == "" {
			ns = r.OwnNamespace
		}
		key := conn.KubeconfigSecretRef.Key
		if key == "" {
			key = "kubeconfig"
		}
		var secret corev1.Secret
		if err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: conn.KubeconfigSecretRef.Name}, &secret); err != nil {
			return nil, fmt.Errorf("read secret %s/%s: %w", ns, conn.KubeconfigSecretRef.Name, err)
		}
		data, ok := secret.Data[key]
		if !ok {
			return nil, fmt.Errorf("secret %s/%s missing key %q", ns, conn.KubeconfigSecretRef.Name, key)
		}
		return clientcmd.RESTConfigFromKubeConfig(data)
	default:
		return nil, fmt.Errorf("connection: exactly one of inCluster or kubeconfigSecretRef is required")
	}
}

// --- condition helpers ---

func (r *ClusterReconciler) setReachable(cr *yafuv1alpha1.Cluster, ok bool, reason, msg string) {
	setCondition(&cr.Status.Conditions, yafuv1alpha1.ConditionReachable, ok, reason, msg, cr.Generation)
}

func (r *ClusterReconciler) setFluxInstalled(cr *yafuv1alpha1.Cluster, ok bool, reason, msg string) {
	setCondition(&cr.Status.Conditions, yafuv1alpha1.ConditionFluxInstalled, ok, reason, msg, cr.Generation)
}

func (r *ClusterReconciler) setReady(cr *yafuv1alpha1.Cluster, ok bool, reason, msg string) {
	setCondition(&cr.Status.Conditions, yafuv1alpha1.ConditionReady, ok, reason, msg, cr.Generation)
}

func setCondition(conds *[]metav1.Condition, t string, ok bool, reason, msg string, gen int64) {
	status := metav1.ConditionTrue
	if !ok {
		status = metav1.ConditionFalse
	}
	now := metav1.Now()
	for i := range *conds {
		if (*conds)[i].Type == t {
			c := &(*conds)[i]
			if c.Status != status {
				c.LastTransitionTime = now
			}
			c.Status = status
			c.Reason = reason
			c.Message = msg
			c.ObservedGeneration = gen
			return
		}
	}
	*conds = append(*conds, metav1.Condition{
		Type:               t,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		LastTransitionTime: now,
		ObservedGeneration: gen,
	})
}

func reachableReason(s cluster.Status) string {
	if s.Reachable {
		return "Connected"
	}
	return "ProbeFailed"
}

func reachableMessage(s cluster.Status) string {
	if s.Reachable {
		return fmt.Sprintf("Kubernetes %s", s.KubernetesVersion)
	}
	if s.LastError != "" {
		return s.LastError
	}
	return "API server unreachable"
}

func fluxReason(s cluster.Status) string {
	if s.FluxInstalled {
		return "Detected"
	}
	if !s.Reachable {
		return "ClusterUnreachable"
	}
	return "FluxNotDetected"
}

func fluxMessage(s cluster.Status) string {
	if s.FluxInstalled {
		return "kustomize.toolkit.fluxcd.io API present"
	}
	if !s.Reachable {
		return "cluster unreachable"
	}
	return "Flux CRDs not found on the target cluster"
}

func readyReason(s cluster.Status) string {
	switch {
	case !s.Reachable:
		return "Unreachable"
	case s.LastError != "":
		return "ProbeError"
	default:
		return "Ready"
	}
}

func readyMessage(s cluster.Status) string {
	if s.LastError != "" {
		return s.LastError
	}
	if !s.Reachable {
		return "cluster unreachable"
	}
	return "cluster registered and reachable"
}

func defaultIfEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
