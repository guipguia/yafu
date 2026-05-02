package watch

import (
	"context"
	"errors"
	"log/slog"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	imageautov1 "github.com/fluxcd/image-automation-controller/api/v1"
	imagereflv1 "github.com/fluxcd/image-reflector-controller/api/v1"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	notificationv1 "github.com/fluxcd/notification-controller/api/v1"
	notificationv1beta3 "github.com/fluxcd/notification-controller/api/v1beta3"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	apiwatch "k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// kindLists maps a kind name to a constructor for an empty ObjectList of
// that kind. We watch every Flux CRD that yafu lists today; the API
// server will return "the server could not find the requested resource"
// for kinds whose CRDs aren't installed and the watcher will skip them.
var kindLists = []struct {
	Kind string
	List func() client.ObjectList
}{
	{"Kustomization", func() client.ObjectList { return &kustomizev1.KustomizationList{} }},
	{"HelmRelease", func() client.ObjectList { return &helmv2.HelmReleaseList{} }},
	{"GitRepository", func() client.ObjectList { return &sourcev1.GitRepositoryList{} }},
	{"HelmRepository", func() client.ObjectList { return &sourcev1.HelmRepositoryList{} }},
	{"OCIRepository", func() client.ObjectList { return &sourcev1.OCIRepositoryList{} }},
	{"Bucket", func() client.ObjectList { return &sourcev1.BucketList{} }},
	{"Alert", func() client.ObjectList { return &notificationv1beta3.AlertList{} }},
	{"Provider", func() client.ObjectList { return &notificationv1beta3.ProviderList{} }},
	{"Receiver", func() client.ObjectList { return &notificationv1.ReceiverList{} }},
	{"ImagePolicy", func() client.ObjectList { return &imagereflv1.ImagePolicyList{} }},
	{"ImageRepository", func() client.ObjectList { return &imagereflv1.ImageRepositoryList{} }},
	{"ImageUpdateAutomation", func() client.ObjectList { return &imageautov1.ImageUpdateAutomationList{} }},
}

// ClusterWatcher fans Kubernetes watch events on the FluxCD CRDs of one
// cluster into the shared Hub. Watch streams reconnect with exponential
// backoff when the API server closes them.
type ClusterWatcher struct {
	Cluster string
	Client  client.WithWatch
	Hub     *Hub
	Logger  *slog.Logger
}

// Run blocks until ctx is cancelled, spawning one goroutine per kind.
func (cw *ClusterWatcher) Run(ctx context.Context) {
	for _, kl := range kindLists {
		kl := kl
		go cw.watchKind(ctx, kl.Kind, kl.List)
	}
	<-ctx.Done()
}

func (cw *ClusterWatcher) watchKind(ctx context.Context, kind string, listCtor func() client.ObjectList) {
	logger := cw.Logger.With("kind", kind)
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		wch, err := cw.Client.Watch(ctx, listCtor())
		if err != nil {
			// Treat "no kind registered" / 404 errors as terminal: the
			// CRD isn't installed on this cluster, no point retrying.
			if isNoKindError(err) {
				logger.Debug("kind not installed; skipping watch")
				return
			}
			logger.Warn("watch start failed; will retry", "err", err, "after", backoff)
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = time.Second
		cw.consume(ctx, kind, wch)
	}
}

func (cw *ClusterWatcher) consume(ctx context.Context, kind string, wch apiwatch.Interface) {
	defer wch.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-wch.ResultChan():
			if !ok {
				return
			}
			if ev.Type == apiwatch.Error {
				cw.Logger.Warn("watch error event", "kind", kind, "object", ev.Object)
				return
			}
			obj, ok := ev.Object.(client.Object)
			if !ok {
				continue
			}
			cw.Hub.Publish(Event{
				Cluster: cw.Cluster,
				Kind:    kind,
				Verb:    string(ev.Type),
				Ns:      obj.GetNamespace(),
				Name:    obj.GetName(),
			})
		}
	}
}

// isNoKindError reports whether err means "the CRD for this kind isn't
// installed on the cluster". meta.IsNoMatchError covers RESTMapper
// failures; the API server's own 404 response carries
// meta.NoKindMatchError too via the discovery cache.
func isNoKindError(err error) bool {
	if err == nil {
		return false
	}
	var nmk *meta.NoKindMatchError
	if errors.As(err, &nmk) {
		return true
	}
	var nrm *meta.NoResourceMatchError
	return errors.As(err, &nrm)
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > 30*time.Second {
		return 30 * time.Second
	}
	return d
}
