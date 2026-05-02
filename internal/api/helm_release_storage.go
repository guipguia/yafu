package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
)

// helmReleaseStorage models the subset of the Helm release object we
// need to walk the rendered manifest. The storage format is documented
// at https://helm.sh/docs/topics/charts/#chart-releases — a base64
// string in the Secret's `release` key, optionally gzip-compressed,
// JSON-encoded inside.
type helmReleaseStorage struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   int    `json:"version"`
	Manifest  string `json:"manifest"` // multi-doc YAML, all rendered objects
}

// helmReleaseInventory pulls the Helm release Secret for the given
// HelmRelease and extracts inventoryRefs from the rendered manifest.
// Returns (refs, note, error). Empty refs + non-empty note signals the
// caller to fall back gracefully (e.g. release not yet installed).
func helmReleaseInventory(ctx context.Context, kube kubernetes.Interface, hr *helmv2.HelmRelease) ([]inventoryRef, string, error) {
	if hr == nil {
		return nil, "", nil
	}
	if len(hr.Status.History) == 0 || hr.Status.History[0] == nil {
		return nil, "no Helm release recorded yet — HelmRelease may not have installed successfully", nil
	}
	snap := hr.Status.History[0]

	storageNS := hr.Status.StorageNamespace
	if storageNS == "" {
		storageNS = snap.Namespace
	}

	secretName := fmt.Sprintf("sh.helm.release.v1.%s.v%d", snap.Name, snap.Version)
	secret, err := kube.CoreV1().Secrets(storageNS).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("Helm release Secret %s/%s not found", storageNS, secretName), nil
		}
		return nil, "", fmt.Errorf("read Helm release secret %s/%s: %w", storageNS, secretName, err)
	}

	rel, err := decodeHelmReleaseSecret(secret)
	if err != nil {
		return nil, "", fmt.Errorf("decode Helm release: %w", err)
	}

	return parseManifestRefs(rel.Manifest, snap.Namespace), "", nil
}

// decodeHelmReleaseSecret undoes Helm's storage encoding. The pipeline
// is: secret.Data["release"] (raw bytes that Helm wrote) → ascii base64
// → optionally gzip-compressed → JSON. Backwards-compat with pre-2.0.0
// releases that didn't compress is preserved by checking for the gzip
// magic bytes before decompressing.
func decodeHelmReleaseSecret(s *corev1.Secret) (*helmReleaseStorage, error) {
	raw, ok := s.Data["release"]
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("secret missing 'release' data key")
	}

	decoded, err := base64.StdEncoding.DecodeString(string(raw))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	if len(decoded) >= 2 && decoded[0] == 0x1f && decoded[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(decoded))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer func() { _ = gz.Close() }()
		decoded, err = io.ReadAll(gz)
		if err != nil {
			return nil, fmt.Errorf("gunzip: %w", err)
		}
	}

	var rel helmReleaseStorage
	if err := json.Unmarshal(decoded, &rel); err != nil {
		return nil, fmt.Errorf("json unmarshal: %w", err)
	}
	return &rel, nil
}

// parseManifestRefs walks Helm's rendered multi-doc YAML and pulls one
// inventoryRef per object. Hooks (annotated with helm.sh/hook) are
// skipped — they aren't part of the steady-state release inventory.
func parseManifestRefs(manifest, defaultNS string) []inventoryRef {
	out := []inventoryRef{}
	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifest), 4096)
	for {
		var u map[string]any
		if err := dec.Decode(&u); err != nil {
			if err == io.EOF {
				break
			}
			// Malformed doc — skip without aborting the whole walk.
			continue
		}
		ref, ok := manifestDocToRef(u, defaultNS)
		if !ok {
			continue
		}
		out = append(out, ref)
	}
	return out
}

func manifestDocToRef(u map[string]any, defaultNS string) (inventoryRef, bool) {
	apiVersion, _ := u["apiVersion"].(string)
	kind, _ := u["kind"].(string)
	if apiVersion == "" || kind == "" {
		return inventoryRef{}, false
	}

	md, _ := u["metadata"].(map[string]any)
	if md == nil {
		return inventoryRef{}, false
	}
	name, _ := md["name"].(string)
	if name == "" {
		return inventoryRef{}, false
	}

	// Skip Helm hooks — they're transient and not part of the release inventory.
	if anns, ok := md["annotations"].(map[string]any); ok {
		if _, isHook := anns["helm.sh/hook"]; isHook {
			return inventoryRef{}, false
		}
	}

	ns, _ := md["namespace"].(string)
	if ns == "" {
		ns = defaultNS
	}

	group, version := "", apiVersion
	if i := strings.Index(apiVersion, "/"); i >= 0 {
		group, version = apiVersion[:i], apiVersion[i+1:]
	}

	return inventoryRef{
		ns:      ns,
		name:    name,
		group:   group,
		kind:    kind,
		version: version,
	}, true
}
