// Package render fetches FluxCD source-controller artifacts and
// renders them (via kustomize-build / helm-template) for the
// Git-vs-cluster diff endpoint.
package render

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// MaxArtifactSize is the maximum tarball size we'll fetch from a
// source-controller artifact URL. The current source-controller
// default cap is 10 MiB; we double it as a safety margin without
// making OOM-by-malicious-source easy.
const MaxArtifactSize = 20 * 1024 * 1024 // 20 MiB

// MaxFileSize bounds the size of any individual extracted file.
// A single-file source bigger than this is almost certainly noise
// or an attempt to fill the temp filesystem.
const MaxFileSize = 8 * 1024 * 1024 // 8 MiB

// FetchTimeout caps the total time spent downloading + extracting
// one artifact. Longer than the API endpoint's own timeout (15s)
// because cold artifacts on slow source-controllers can take a
// few seconds to seek + serve.
const FetchTimeout = 25 * time.Second

// ArtifactRef carries everything FetchAndExtract needs about a
// single source-controller artifact. The Digest is optional but
// strongly recommended — when present, the fetcher verifies the
// downloaded blob matches before extracting, so a man-in-the-middle
// or a half-cooked source-controller response can't drive a render.
type ArtifactRef struct {
	URL    string
	Digest string // "sha256:<hex>" or just "<hex>"; empty skips verification
}

// FetchAndExtract downloads and extracts the artifact referenced by
// ref into a fresh temp directory. The returned cleanup function
// must be called by the caller (defer is fine) — it removes the
// extracted tree.
//
// The fetcher enforces three guarantees:
//   - downloads cap at MaxArtifactSize bytes (HTTP-side)
//   - per-file extraction cap at MaxFileSize bytes
//   - tar entries with absolute paths or "..` segments are rejected
//
// Returns the absolute path of the extraction root. The caller
// should treat that path as the "repo root" for kustomize/helm.
func FetchAndExtract(ctx context.Context, ref ArtifactRef) (root string, cleanup func() error, err error) {
	if ref.URL == "" {
		return "", nil, errors.New("artifact URL is empty")
	}
	parsed, err := url.Parse(ref.URL)
	if err != nil {
		return "", nil, fmt.Errorf("parse artifact URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("unsupported scheme %q (want http/https)", parsed.Scheme)
	}

	fetchCtx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(fetchCtx, http.MethodGet, ref.URL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("fetch artifact: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("artifact fetch returned %d", resp.StatusCode)
	}

	// Read into memory so we can verify the digest before touching
	// the filesystem. MaxArtifactSize+1 lets us detect the overflow
	// case and reject without silently truncating.
	limited := io.LimitReader(resp.Body, MaxArtifactSize+1)
	blob, err := io.ReadAll(limited)
	if err != nil {
		return "", nil, fmt.Errorf("read artifact body: %w", err)
	}
	if len(blob) > MaxArtifactSize {
		return "", nil, fmt.Errorf("artifact exceeds %d byte cap", MaxArtifactSize)
	}

	if ref.Digest != "" {
		if err := verifyDigest(blob, ref.Digest); err != nil {
			return "", nil, err
		}
	}

	dir, err := os.MkdirTemp("", "yafu-render-*")
	if err != nil {
		return "", nil, fmt.Errorf("mkdir temp: %w", err)
	}
	cleanup = func() error { return os.RemoveAll(dir) }

	if err := extractTarGz(blob, dir); err != nil {
		_ = cleanup()
		return "", nil, fmt.Errorf("extract artifact: %w", err)
	}
	return dir, cleanup, nil
}

// verifyDigest accepts both bare hex and the canonical "sha256:..." form.
func verifyDigest(blob []byte, digest string) error {
	want := strings.TrimPrefix(digest, "sha256:")
	got := sha256.Sum256(blob)
	if !strings.EqualFold(hex.EncodeToString(got[:]), want) {
		return fmt.Errorf("artifact digest mismatch (want %s, got %s)", want, hex.EncodeToString(got[:]))
	}
	return nil
}

// extractTarGz writes a gzipped tar blob to dir, refusing absolute
// paths and parent-directory segments. Files are clipped to
// MaxFileSize to bound extraction work even within a tar that
// passed the outer MaxArtifactSize check.
func extractTarGz(blob []byte, dir string) error {
	gz, err := gzip.NewReader(bytesReader(blob))
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		// Tar paths use POSIX semantics regardless of host OS, so we
		// validate with the `path` package before joining via filepath.
		if path.IsAbs(hdr.Name) {
			return fmt.Errorf("tar entry %q is absolute", hdr.Name)
		}
		clean := path.Clean(hdr.Name)
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("tar entry %q escapes root", hdr.Name)
		}
		dst := filepath.Join(dir, filepath.FromSlash(clean))
		// Defence in depth — even after Clean+Join, ensure the
		// final path stays under dir.
		if rel, err := filepath.Rel(dir, dst); err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("tar entry %q escapes root after join", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", dst, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return fmt.Errorf("mkdir parent of %s: %w", dst, err)
			}
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return fmt.Errorf("open %s: %w", dst, err)
			}
			// LimitReader caps each file at MaxFileSize+1 so we can
			// detect truncation without writing partial garbage.
			n, err := io.Copy(f, io.LimitReader(tr, MaxFileSize+1))
			closeErr := f.Close()
			if err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}
			if closeErr != nil {
				return fmt.Errorf("close %s: %w", dst, closeErr)
			}
			if n > MaxFileSize {
				return fmt.Errorf("tar entry %q exceeds %d byte file cap", hdr.Name, MaxFileSize)
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Symlinks in source artifacts can point anywhere on the
			// filesystem the renderer reads. Skip them — kustomize/helm
			// don't need them for the manifests we care about.
			continue
		default:
			// Devices, fifos, etc. are not interesting for source artifacts.
			continue
		}
	}
}

// bytesReader is a tiny helper to avoid pulling in bytes.NewReader
// just for two callers; keeps the import list lean.
func bytesReader(b []byte) *byteReader { return &byteReader{b: b} }

type byteReader struct {
	b   []byte
	off int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}
