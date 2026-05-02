package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxmeta "github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"go.opentelemetry.io/otel/attribute"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apitypes "github.com/guipguia/yafu/internal/api/types"
	"github.com/guipguia/yafu/internal/auth"
	"github.com/guipguia/yafu/internal/cluster"
	"github.com/guipguia/yafu/internal/render"
	"github.com/guipguia/yafu/internal/tracing"
)

// renderTimeout caps the total time spent fetching + rendering one
// application. Generous because real charts can be slow to template
// (subcharts, large values), but bounded to keep the API endpoint
// responsive when source-controller is unhealthy.
const renderTimeout = 30 * time.Second

// render handles GET /api/v1/applications/.../render. It produces
// the structured Git-vs-cluster diff: fetch the application's
// source artifact, render via kustomize-build / helm-template, diff
// each rendered resource against live cluster state, return an
// apitypes.RenderResponse.
//
// Top-level errors (artifact missing, source not ready, build
// failure) are returned as 200 with response.error set, so the
// frontend renders the structured "render failed" state rather
// than a generic HTTP error. Per-resource fetch failures are
// captured on individual rows.
func (h *applicationsHandler) render(w http.ResponseWriter, r *http.Request) {
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

	ctx, cancel := context.WithTimeout(r.Context(), renderTimeout)
	defer cancel()

	resp := apitypes.RenderResponse{
		AppID:      appID(clusterID, ns, kind, name),
		Resources:  []apitypes.RenderResource{},
		RenderedAt: time.Now().UTC().Format(time.RFC3339),
	}

	switch kind {
	case "Kustomization":
		var k kustomizev1.Kustomization
		if err := e.Client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &k); err != nil {
			writeError(w, statusFromErr(err), err.Error())
			return
		}
		if err := renderKustomization(ctx, e, &k, &resp); err != nil {
			resp.Error = err.Error()
		}
	case "HelmRelease":
		var hr helmv2.HelmRelease
		if err := e.Client.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, &hr); err != nil {
			writeError(w, statusFromErr(err), err.Error())
			return
		}
		if err := renderHelmRelease(ctx, e, &hr, &resp); err != nil {
			resp.Error = err.Error()
		}
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported kind %q", kind))
		return
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func renderKustomization(ctx context.Context, e *cluster.Entry, k *kustomizev1.Kustomization, resp *apitypes.RenderResponse) error {
	ctx, span := tracing.Tracer().Start(ctx, "render.kustomization")
	defer span.End()
	span.SetAttributes(
		attribute.String("yafu.cluster", e.Name),
		attribute.String("yafu.namespace", k.Namespace),
		attribute.String("yafu.name", k.Name),
	)

	resp.Source.Method = "kustomize build"

	src, srcKind, ref, err := getKustomizationSource(ctx, e.Client, k)
	if err != nil {
		return err
	}
	resp.Source.Name = src.GetName()
	resp.Source.Namespace = src.GetNamespace()
	resp.Source.Kind = srcKind
	resp.Source.Ref = ref

	art := src.GetArtifact()
	if art == nil {
		return fmt.Errorf("source %s/%s has no artifact yet — wait for the source to reconcile", src.GetNamespace(), src.GetName())
	}
	resp.Source.Revision = art.Revision
	span.SetAttributes(attribute.String("yafu.source.revision", art.Revision))

	fetchCtx, fetchSpan := tracing.Tracer().Start(ctx, "render.fetchArtifact")
	dir, cleanup, err := render.FetchAndExtract(fetchCtx, render.ArtifactRef{
		URL:    art.URL,
		Digest: art.Digest,
	})
	fetchSpan.End()
	if err != nil {
		return fmt.Errorf("fetch artifact: %w", err)
	}
	defer func() { _ = cleanup() }()

	buildCtx, buildSpan := tracing.Tracer().Start(ctx, "render.kustomizeBuild")
	rendered, err := render.RenderKustomization(buildCtx, dir, k)
	buildSpan.SetAttributes(attribute.Int("yafu.rendered.count", len(rendered)))
	buildSpan.End()
	if err != nil {
		return fmt.Errorf("kustomize build: %w", err)
	}

	diffCtx, diffSpan := tracing.Tracer().Start(ctx, "render.diff")
	resources, err := render.DiffResources(diffCtx, e.Client, render.DiffOptions{
		Rendered:  rendered,
		Inventory: inventoryKeysFromKustomization(k),
	})
	diffSpan.End()
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	resp.Resources = resources
	return nil
}

func renderHelmRelease(ctx context.Context, e *cluster.Entry, hr *helmv2.HelmRelease, resp *apitypes.RenderResponse) error {
	ctx, span := tracing.Tracer().Start(ctx, "render.helmRelease")
	defer span.End()
	span.SetAttributes(
		attribute.String("yafu.cluster", e.Name),
		attribute.String("yafu.namespace", hr.Namespace),
		attribute.String("yafu.name", hr.Name),
	)

	resp.Source.Method = "helm template"

	fetchCtx, fetchSpan := tracing.Tracer().Start(ctx, "render.fetchChartArtifact")
	chartDir, srcInfo, cleanup, err := materializeHelmChart(fetchCtx, e.Client, hr)
	fetchSpan.End()
	if err != nil {
		return err
	}
	defer func() { _ = cleanup() }()

	resp.Source.Name = srcInfo.name
	resp.Source.Namespace = srcInfo.namespace
	resp.Source.Kind = srcInfo.kind
	resp.Source.Ref = srcInfo.ref
	resp.Source.Revision = srcInfo.revision
	span.SetAttributes(attribute.String("yafu.source.revision", srcInfo.revision))

	tmplCtx, tmplSpan := tracing.Tracer().Start(ctx, "render.helmTemplate")
	rendered, err := render.RenderHelmRelease(tmplCtx, chartDir, hr)
	tmplSpan.SetAttributes(attribute.Int("yafu.rendered.count", len(rendered)))
	tmplSpan.End()
	if err != nil {
		return fmt.Errorf("helm template: %w", err)
	}

	// Inventory comes from the existing Helm-storage decoder so we
	// can detect "extra-on-cluster".
	invRefs, _, _ := helmReleaseInventory(ctx, e.Kube, hr)
	inv := make([]render.ResourceKey, 0, len(invRefs))
	for _, r := range invRefs {
		inv = append(inv, render.ResourceKey{
			GVK: schema.GroupVersionKind{Group: r.group, Version: r.version, Kind: r.kind},
			Ns:  r.ns,
			Nm:  r.name,
		})
	}

	diffCtx, diffSpan := tracing.Tracer().Start(ctx, "render.diff")
	resources, err := render.DiffResources(diffCtx, e.Client, render.DiffOptions{
		Rendered:  rendered,
		Inventory: inv,
	})
	diffSpan.End()
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	resp.Resources = resources
	return nil
}

// fluxSource is a small interface that lets us treat the source
// kinds (GitRepository / OCIRepository / Bucket) uniformly when
// reading their artifact + revision metadata.
type fluxSource interface {
	GetName() string
	GetNamespace() string
	GetArtifact() *fluxmeta.Artifact
}

type gitRepoAdapter struct{ *sourcev1.GitRepository }

func (a gitRepoAdapter) GetArtifact() *fluxmeta.Artifact { return a.Status.Artifact }

type ociRepoAdapter struct{ *sourcev1.OCIRepository }

func (a ociRepoAdapter) GetArtifact() *fluxmeta.Artifact { return a.Status.Artifact }

type bucketAdapter struct{ *sourcev1.Bucket }

func (a bucketAdapter) GetArtifact() *fluxmeta.Artifact { return a.Status.Artifact }

func getKustomizationSource(ctx context.Context, c client.Client, k *kustomizev1.Kustomization) (fluxSource, string, string, error) {
	srcNs := k.Spec.SourceRef.Namespace
	if srcNs == "" {
		srcNs = k.Namespace
	}
	key := types.NamespacedName{Namespace: srcNs, Name: k.Spec.SourceRef.Name}
	switch k.Spec.SourceRef.Kind {
	case "GitRepository":
		var g sourcev1.GitRepository
		if err := c.Get(ctx, key, &g); err != nil {
			return nil, "", "", fmt.Errorf("get GitRepository %s: %w", key, err)
		}
		return gitRepoAdapter{&g}, "GitRepository", gitRefDescription(g.Spec.Reference), nil
	case "OCIRepository":
		var o sourcev1.OCIRepository
		if err := c.Get(ctx, key, &o); err != nil {
			return nil, "", "", fmt.Errorf("get OCIRepository %s: %w", key, err)
		}
		return ociRepoAdapter{&o}, "OCIRepository", ociRepoRef(o.Spec.Reference), nil
	case "Bucket":
		var b sourcev1.Bucket
		if err := c.Get(ctx, key, &b); err != nil {
			return nil, "", "", fmt.Errorf("get Bucket %s: %w", key, err)
		}
		return bucketAdapter{&b}, "Bucket", "", nil
	default:
		return nil, "", "", fmt.Errorf("unsupported source kind %q", k.Spec.SourceRef.Kind)
	}
}

type helmSourceInfo struct {
	name, namespace, kind, ref, revision string
}

// materializeHelmChart fetches + extracts the chart artifact for
// the HelmRelease via its status.helmChart pointer, returns the
// chart directory (containing Chart.yaml) and a cleanup function.
func materializeHelmChart(ctx context.Context, c client.Client, hr *helmv2.HelmRelease) (string, helmSourceInfo, func() error, error) {
	noop := func() error { return nil }
	if hr.Status.HelmChart == "" {
		return "", helmSourceInfo{}, noop,
			fmt.Errorf("HelmRelease has no status.helmChart yet — wait for the release to reconcile at least once")
	}

	parts := strings.SplitN(hr.Status.HelmChart, "/", 2)
	if len(parts) != 2 {
		return "", helmSourceInfo{}, noop,
			fmt.Errorf("malformed HelmRelease.status.helmChart %q", hr.Status.HelmChart)
	}
	hcNs, hcName := parts[0], parts[1]

	var hc sourcev1.HelmChart
	if err := c.Get(ctx, types.NamespacedName{Namespace: hcNs, Name: hcName}, &hc); err != nil {
		return "", helmSourceInfo{}, noop, fmt.Errorf("get HelmChart %s/%s: %w", hcNs, hcName, err)
	}
	if hc.Status.Artifact == nil {
		return "", helmSourceInfo{}, noop, fmt.Errorf("HelmChart %s/%s has no artifact yet", hcNs, hcName)
	}

	dir, cleanup, err := render.FetchAndExtract(ctx, render.ArtifactRef{
		URL:    hc.Status.Artifact.URL,
		Digest: hc.Status.Artifact.Digest,
	})
	if err != nil {
		return "", helmSourceInfo{}, noop, fmt.Errorf("fetch chart artifact: %w", err)
	}

	chartDir, err := findChartRoot(dir)
	if err != nil {
		_ = cleanup()
		return "", helmSourceInfo{}, noop, err
	}

	info := helmSourceInfo{
		name:      hc.Spec.SourceRef.Name,
		namespace: hcNs,
		kind:      hc.Spec.SourceRef.Kind,
		ref:       hc.Spec.Version,
		revision:  hc.Status.Artifact.Revision,
	}
	return chartDir, info, cleanup, nil
}

// findChartRoot locates the directory containing Chart.yaml inside
// an extracted artifact. source-controller's chart artifacts often
// nest the chart inside a single subdirectory named after the
// chart, so we look one level deep when the root doesn't have
// Chart.yaml directly.
func findChartRoot(dir string) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "Chart.yaml")); err == nil {
		return dir, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("scan extracted artifact: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if _, err := os.Stat(filepath.Join(sub, "Chart.yaml")); err == nil {
			return sub, nil
		}
	}
	return "", fmt.Errorf("no Chart.yaml found under %s", dir)
}

// inventoryKeysFromKustomization translates Flux's status inventory
// IDs to render.ResourceKey for diff comparison.
func inventoryKeysFromKustomization(k *kustomizev1.Kustomization) []render.ResourceKey {
	if k.Status.Inventory == nil {
		return nil
	}
	out := make([]render.ResourceKey, 0, len(k.Status.Inventory.Entries))
	for _, ref := range k.Status.Inventory.Entries {
		// ID format: "<ns>_<name>_<group>_<kind>"
		parts := strings.SplitN(ref.ID, "_", 4)
		if len(parts) != 4 {
			continue
		}
		out = append(out, render.ResourceKey{
			GVK: schema.GroupVersionKind{Group: parts[2], Version: ref.Version, Kind: parts[3]},
			Ns:  parts[0],
			Nm:  parts[1],
		})
	}
	return out
}

func gitRefDescription(ref *sourcev1.GitRepositoryRef) string {
	if ref == nil {
		return ""
	}
	switch {
	case ref.Branch != "":
		return ref.Branch
	case ref.Tag != "":
		return ref.Tag
	case ref.SemVer != "":
		return ref.SemVer
	case ref.Name != "":
		return ref.Name
	case ref.Commit != "":
		return ref.Commit
	}
	return ""
}

func statusFromErr(err error) int {
	if apierrors.IsNotFound(err) {
		return http.StatusNotFound
	}
	if apierrors.IsForbidden(err) {
		return http.StatusForbidden
	}
	return http.StatusInternalServerError
}

