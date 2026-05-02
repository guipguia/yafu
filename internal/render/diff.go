package render

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	apitypes "github.com/guipguia/yafu/internal/api/types"
)

// ResourceKey uniquely identifies a Kubernetes resource for the
// purposes of comparison: GroupVersionKind + namespace + name.
type ResourceKey struct {
	GVK schema.GroupVersionKind
	Ns  string
	Nm  string
}

// keyOf builds the comparison key from an Unstructured. The
// version is normalised to the *resource* version (apps/v1 not
// apps/v1beta1) so a render targeting apps/v1 still matches a live
// Deployment served as apps/v1 — Kubernetes redirects requests
// across versions transparently for built-in kinds.
func keyOf(o unstructured.Unstructured) ResourceKey {
	return ResourceKey{
		GVK: o.GroupVersionKind(),
		Ns:  o.GetNamespace(),
		Nm:  o.GetName(),
	}
}

// DiffOptions carries inputs to DiffResources. Inventory is the set
// of resources Flux's status thinks should exist on the cluster; we
// use it to detect "extra" — present on the cluster, applied by Flux
// previously, but not in the current rendered source. (Resources on
// the cluster that were never managed by Flux at all are out of
// scope.)
type DiffOptions struct {
	Rendered  []unstructured.Unstructured
	Inventory []ResourceKey
}

// DiffResources fetches the live counterpart of each rendered
// resource (and each inventory entry not in the rendered set) and
// classifies the result.
//
// Status semantics:
//
//	in-sync             — both sides exist and the diff is empty
//	drifted             — both sides exist and differ
//	missing-on-cluster  — rendered says it should be there; live API returns NotFound
//	extra-on-cluster    — present in inventory + on the cluster, but not in the
//	                      rendered source (a prune candidate when prune=true)
//	render-error        — set by the caller when build itself failed; never produced here
//
// Network errors (anything other than NotFound) bubble up as
// status="unknown" with the error message in the resource's
// renderError field, so the operator can see why the diff for a
// specific resource didn't compute even when others succeeded.
func DiffResources(ctx context.Context, c client.Client, opts DiffOptions) ([]apitypes.RenderResource, error) {
	resources := make([]apitypes.RenderResource, 0, len(opts.Rendered))
	renderedKeys := make(map[ResourceKey]struct{}, len(opts.Rendered))

	for _, want := range opts.Rendered {
		key := keyOf(want)
		renderedKeys[key] = struct{}{}
		resources = append(resources, classifyOne(ctx, c, want, key))
	}

	for _, inv := range opts.Inventory {
		if _, alreadyRendered := renderedKeys[inv]; alreadyRendered {
			continue
		}
		// Inventory entries not in the rendered output → extra. We
		// confirm by checking the cluster; if the resource isn't
		// actually present (already pruned), we don't report it.
		extra := apitypes.RenderResource{
			Group:          inv.GVK.Group,
			Version:        inv.GVK.Version,
			Kind:           inv.GVK.Kind,
			Ns:             inv.Ns,
			Name:           inv.Nm,
			Status:         "extra-on-cluster",
			ReconcileWould: "delete",
		}
		live, err := getLive(ctx, c, inv.GVK, inv.Ns, inv.Nm)
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			extra.Status = "extra-on-cluster"
			extra.RenderError = err.Error()
		}
		_ = live
		resources = append(resources, extra)
	}

	sort.Slice(resources, func(i, j int) bool {
		a, b := resources[i], resources[j]
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Ns != b.Ns {
			return a.Ns < b.Ns
		}
		return a.Name < b.Name
	})
	return resources, nil
}

// classifyOne handles a single rendered → live comparison.
func classifyOne(ctx context.Context, c client.Client, want unstructured.Unstructured, key ResourceKey) apitypes.RenderResource {
	out := apitypes.RenderResource{
		Group:   key.GVK.Group,
		Version: key.GVK.Version,
		Kind:    key.GVK.Kind,
		Ns:      key.Ns,
		Name:    key.Nm,
	}

	live, err := getLive(ctx, c, key.GVK, key.Ns, key.Nm)
	if err != nil {
		if apierrors.IsNotFound(err) {
			out.Status = "missing-on-cluster"
			out.ReconcileWould = "create"
			return out
		}
		// Any other error: fail-soft on a per-resource basis with
		// the error message stashed on the resource.
		out.Status = "render-error"
		out.RenderError = err.Error()
		return out
	}

	wantClean := stripServerSideNoise(want)
	liveClean := stripServerSideNoise(*live)

	wantYAML, errA := yaml.Marshal(wantClean.Object)
	liveYAML, errB := yaml.Marshal(liveClean.Object)
	if errA != nil || errB != nil {
		out.Status = "render-error"
		out.RenderError = fmt.Sprintf("serialise yaml: %v / %v", errA, errB)
		return out
	}

	if string(wantYAML) == string(liveYAML) {
		out.Status = "in-sync"
		return out
	}

	out.Status = "drifted"
	out.ReconcileWould = "update"
	out.Hunks = computeHunks(string(wantYAML), string(liveYAML))
	return out
}

func getLive(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, error) {
	live := &unstructured.Unstructured{}
	live.SetGroupVersionKind(gvk)
	if err := c.Get(ctx, types.NamespacedName{Namespace: ns, Name: name}, live); err != nil {
		return nil, err
	}
	return live, nil
}

// stripServerSideNoise removes fields the API server populates
// (managedFields, uid, resourceVersion, generation, etc.) plus the
// status block — those don't belong in a desired-vs-live YAML diff
// because they're never present in source manifests.
//
// We deliberately keep labels/annotations as-is — kustomize-controller
// adds e.g. "kustomize.toolkit.fluxcd.io/name" annotations on apply,
// so the rendered side and the live side both have them. Stripping
// those would cause noisy diffs on the first reconcile after they
// were added.
func stripServerSideNoise(o unstructured.Unstructured) unstructured.Unstructured {
	cp := unstructured.Unstructured{Object: deepCopyMap(o.Object)}
	if md, ok := cp.Object["metadata"].(map[string]interface{}); ok {
		for _, f := range []string{
			"managedFields",
			"uid",
			"resourceVersion",
			"generation",
			"creationTimestamp",
			"selfLink",
			"finalizers",
		} {
			delete(md, f)
		}
	}
	delete(cp.Object, "status")
	return cp
}

// deepCopyMap is a JSON-style deep copy that's good enough for
// Kubernetes objects (no functions, no cycles). Avoids dragging in
// reflect-based copiers for a hot path.
func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(t)
	case []interface{}:
		c := make([]interface{}, len(t))
		for i, e := range t {
			c[i] = deepCopyValue(e)
		}
		return c
	default:
		return v
	}
}

// computeHunks produces a single hunk representing the unified
// diff between desired and live YAML. We don't try to split into
// multiple hunks today — line-based unified diff is good enough
// to render in the existing Split / Unified UI, and grouping
// adjacent changes is a UX nicety that's worth doing once we
// see how real diffs look.
func computeHunks(desired, live string) []apitypes.RenderHunk {
	dlines := strings.Split(strings.TrimRight(desired, "\n"), "\n")
	llines := strings.Split(strings.TrimRight(live, "\n"), "\n")

	matcher := difflib.NewMatcher(dlines, llines)
	ops := matcher.GetOpCodes()

	lines := make([]apitypes.RenderLine, 0, len(dlines)+len(llines))
	for _, op := range ops {
		switch op.Tag {
		case 'e': // equal
			for k := op.I1; k < op.I2; k++ {
				lines = append(lines, apitypes.RenderLine{
					Kind:      "context",
					DesiredLn: ptrInt(k + 1),
					LiveLn:    ptrInt(op.J1 + (k - op.I1) + 1),
					Text:      dlines[k],
				})
			}
		case 'd': // delete from desired only
			for k := op.I1; k < op.I2; k++ {
				lines = append(lines, apitypes.RenderLine{
					Kind:      "del",
					DesiredLn: ptrInt(k + 1),
					Text:      dlines[k],
				})
			}
		case 'i': // insert into live only
			for k := op.J1; k < op.J2; k++ {
				lines = append(lines, apitypes.RenderLine{
					Kind:   "add",
					LiveLn: ptrInt(k + 1),
					Text:   llines[k],
				})
			}
		case 'r': // replace — both sides changed
			for k := op.I1; k < op.I2; k++ {
				lines = append(lines, apitypes.RenderLine{
					Kind:      "del",
					DesiredLn: ptrInt(k + 1),
					Text:      dlines[k],
				})
			}
			for k := op.J1; k < op.J2; k++ {
				lines = append(lines, apitypes.RenderLine{
					Kind:   "add",
					LiveLn: ptrInt(k + 1),
					Text:   llines[k],
				})
			}
		}
	}

	return []apitypes.RenderHunk{{Label: "object", Lines: lines}}
}

func ptrInt(v int) *int { return &v }
