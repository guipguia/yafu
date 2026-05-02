package render

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RenderHelmRelease runs `helm template` against a chart unpacked
// at chartDir, using the values inlined in the HelmRelease spec.
// The result is parsed into Unstructured objects matching what
// helm-controller would apply.
//
// Limitations (deferred to follow-up commits):
//   - valuesFrom (ConfigMap/Secret references) — needs cluster
//     reads + key lookup
//   - postRenderers — yafu doesn't render the post-render YAML
//   - chart dependencies (subcharts) work through helm.sh/helm/v3
//     loader.Load when the artifact contains charts/<dep>/, but
//     dependencies declared via Chart.yaml that need a separate
//     resolve step are out of scope today
//
// chartDir is expected to point directly at the chart root (the
// directory containing Chart.yaml), not a parent. The caller
// (api.handler) is responsible for finding the chart subdir
// inside the artifact tree before invoking this function.
func RenderHelmRelease(ctx context.Context, chartDir string, hr *helmv2.HelmRelease) ([]unstructured.Unstructured, error) {
	if hr == nil {
		return nil, fmt.Errorf("nil helm release")
	}

	chrt, err := loader.Load(chartDir)
	if err != nil {
		return nil, fmt.Errorf("load chart at %s: %w", chartDir, err)
	}

	values := map[string]interface{}{}
	if hr.Spec.Values != nil && len(hr.Spec.Values.Raw) > 0 {
		if err := json.Unmarshal(hr.Spec.Values.Raw, &values); err != nil {
			return nil, fmt.Errorf("decode HelmRelease spec.values: %w", err)
		}
	}

	releaseName := hr.GetReleaseName()
	if releaseName == "" {
		releaseName = hr.Name
	}
	releaseNs := hr.GetReleaseNamespace()
	if releaseNs == "" {
		releaseNs = hr.Namespace
	}

	rv, err := chartutil.ToRenderValues(chrt, values, chartutil.ReleaseOptions{
		Name:      releaseName,
		Namespace: releaseNs,
		IsInstall: true,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("compose render values: %w", err)
	}

	rendered, err := engine.Render(chrt, rv)
	if err != nil {
		return nil, fmt.Errorf("helm template: %w", err)
	}

	return parseRenderedFiles(rendered, releaseNs)
}

// parseRenderedFiles takes engine.Render's output (path → YAML
// content) and produces one Unstructured per non-empty document.
// Empty templates and pure-comment files are skipped.
//
// The default namespace is applied to namespaced objects that
// don't already carry one, matching the helm CLI's
// `--namespace` semantics.
func parseRenderedFiles(rendered map[string]string, defaultNs string) ([]unstructured.Unstructured, error) {
	out := make([]unstructured.Unstructured, 0, len(rendered))
	for fp, body := range rendered {
		// helm renders NOTES.txt and partials too — skip anything
		// that isn't a manifest. Templates produce .yaml/.yml.
		ext := strings.ToLower(filepath.Ext(fp))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		objs, err := splitYAMLDocuments([]byte(body))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", fp, err)
		}
		for i := range objs {
			obj := &objs[i]
			if defaultNs != "" && !isClusterScoped(obj.GetKind()) && obj.GetNamespace() == "" {
				obj.SetNamespace(defaultNs)
			}
			out = append(out, *obj)
		}
	}
	return out, nil
}
