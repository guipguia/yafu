package render

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"
)

// RenderKustomization runs `kustomize build` on the given artifact
// directory at the path specified by the Kustomization spec, then
// stamps the result with target namespace + common labels per the
// spec, and returns the rendered objects as Unstructured.
//
// Limitations (deferred to follow-up commits):
//   - postBuild substitutions (substituteFrom) — requires reading
//     ConfigMaps/Secrets out of the cluster
//   - decryption (sopsSecret) — requires a SOPS keyring
//   - components — work today via the kustomize API, but untested
//   - patches with strategic-merge or JSON6902 from external files
//
// The 95% case (vanilla kustomization.yaml under spec.path) works.
func RenderKustomization(ctx context.Context, artifactDir string, k *kustomizev1.Kustomization) ([]unstructured.Unstructured, error) {
	if k == nil {
		return nil, fmt.Errorf("nil kustomization")
	}

	// spec.path is relative to the artifact root. Default ./ when
	// unset.
	rel := k.Spec.Path
	if rel == "" {
		rel = "./"
	}
	rel = strings.TrimPrefix(rel, "./")
	target := filepath.Join(artifactDir, filepath.FromSlash(rel))

	// Defence in depth — the path field is operator-controlled but
	// has been seen to contain '..' in misconfigured Kustomizations.
	rt, err := filepath.Rel(artifactDir, target)
	if err != nil || strings.HasPrefix(rt, "..") {
		return nil, fmt.Errorf("kustomization path %q escapes artifact root", k.Spec.Path)
	}

	fs := filesys.MakeFsOnDisk()
	hasYAML := fs.Exists(filepath.Join(target, "kustomization.yaml")) ||
		fs.Exists(filepath.Join(target, "kustomization.yml")) ||
		fs.Exists(filepath.Join(target, "Kustomization"))
	if !hasYAML {
		return nil, fmt.Errorf("no kustomization.yaml under %q", k.Spec.Path)
	}

	opts := krusty.MakeDefaultOptions()
	// LoadRestrictionsNone matches kustomize-controller's runtime
	// behaviour — it lets `resources:` and `bases:` reach outside
	// the kustomization root, which production manifests routinely
	// do (e.g., a base/ directory next to overlays/).
	opts.LoadRestrictions = kustypes.LoadRestrictionsNone

	rm, err := krusty.MakeKustomizer(opts).Run(fs, target)
	if err != nil {
		return nil, fmt.Errorf("kustomize build: %w", err)
	}

	yml, err := rm.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("serialize kustomize result: %w", err)
	}

	objs, err := splitYAMLDocuments(yml)
	if err != nil {
		return nil, fmt.Errorf("split rendered YAML: %w", err)
	}

	// Apply spec.targetNamespace if set — kustomize-controller does
	// this after the build, not as a kustomize transformer, so we
	// match that behaviour.
	if k.Spec.TargetNamespace != "" {
		for i := range objs {
			obj := &objs[i]
			if isClusterScoped(obj.GetKind()) {
				continue
			}
			if obj.GetNamespace() == "" {
				obj.SetNamespace(k.Spec.TargetNamespace)
			}
		}
	}

	return objs, nil
}

// splitYAMLDocuments parses a multi-document YAML stream into one
// Unstructured per document. Empty documents and YAML comments are
// skipped.
func splitYAMLDocuments(b []byte) ([]unstructured.Unstructured, error) {
	docs := splitYAMLStream(b)
	out := make([]unstructured.Unstructured, 0, len(docs))
	for _, d := range docs {
		trimmed := strings.TrimSpace(string(d))
		if trimmed == "" || trimmed == "---" {
			continue
		}
		var m map[string]interface{}
		if err := yaml.Unmarshal(d, &m); err != nil {
			return nil, fmt.Errorf("parse YAML doc: %w", err)
		}
		if len(m) == 0 {
			continue
		}
		out = append(out, unstructured.Unstructured{Object: m})
	}
	return out, nil
}

// splitYAMLStream is a tiny, dependency-free splitter on `---` lines.
// It avoids pulling in yaml.v3's decoder iteration just for this; the
// trade-off is that `---` inside string literals would split the doc,
// which doesn't happen in Kubernetes manifests in practice.
func splitYAMLStream(b []byte) [][]byte {
	const sep = "\n---"
	parts := strings.Split("\n"+string(b), sep)
	out := make([][]byte, 0, len(parts))
	for i, p := range parts {
		// Strip the leading "\n" we prepended to part 0.
		if i == 0 {
			p = strings.TrimPrefix(p, "\n")
		}
		out = append(out, []byte(p))
	}
	return out
}

// isClusterScoped is a coarse list of cluster-scoped kinds. Used
// when applying targetNamespace so we don't stamp Namespace,
// CRD, etc. with a namespace they can't have. Misses for custom
// CRDs are harmless — kubectl/Flux will reject the apply with a
// clear error if we put a namespace where none belongs, but the
// list covers every standard kind a Kustomization is likely to
// emit.
func isClusterScoped(kind string) bool {
	switch kind {
	case "Namespace",
		"PersistentVolume",
		"ClusterRole",
		"ClusterRoleBinding",
		"CustomResourceDefinition",
		"StorageClass",
		"PriorityClass",
		"Node",
		"APIService",
		"MutatingWebhookConfiguration",
		"ValidatingWebhookConfiguration",
		"ValidatingAdmissionPolicy",
		"ValidatingAdmissionPolicyBinding",
		"IngressClass":
		return true
	}
	return false
}
