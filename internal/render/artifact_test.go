package render

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchAndExtract_HappyPath(t *testing.T) {
	blob := buildTar(t, map[string]string{
		"kustomize/kustomization.yaml": "resources:\n- deployment.yaml\n",
		"kustomize/deployment.yaml":    "apiVersion: apps/v1\nkind: Deployment\n",
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(blob)
	}))
	defer srv.Close()

	digest := sha256.Sum256(blob)
	root, cleanup, err := FetchAndExtract(context.Background(), ArtifactRef{
		URL:    srv.URL,
		Digest: "sha256:" + hex.EncodeToString(digest[:]),
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	defer func() { _ = cleanup() }()

	got, err := os.ReadFile(filepath.Join(root, "kustomize", "deployment.yaml"))
	if err != nil {
		t.Fatalf("read extracted file: %v", err)
	}
	if !strings.Contains(string(got), "Deployment") {
		t.Errorf("unexpected content: %s", got)
	}
}

func TestFetchAndExtract_DigestMismatch(t *testing.T) {
	blob := buildTar(t, map[string]string{"x.yaml": "x"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(blob)
	}))
	defer srv.Close()

	_, _, err := FetchAndExtract(context.Background(), ArtifactRef{
		URL:    srv.URL,
		Digest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err == nil {
		t.Fatal("expected digest mismatch error")
	}
	if !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestFetchAndExtract_RejectsAbsolutePath(t *testing.T) {
	blob := buildTar(t, map[string]string{"/etc/passwd": "evil"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(blob)
	}))
	defer srv.Close()

	_, _, err := FetchAndExtract(context.Background(), ArtifactRef{URL: srv.URL})
	if err == nil {
		t.Fatal("expected rejection of absolute path")
	}
}

func TestFetchAndExtract_RejectsParentTraversal(t *testing.T) {
	blob := buildTar(t, map[string]string{"../escape.yaml": "no"})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(blob)
	}))
	defer srv.Close()

	_, _, err := FetchAndExtract(context.Background(), ArtifactRef{URL: srv.URL})
	if err == nil {
		t.Fatal("expected rejection of .. path")
	}
}

func TestFetchAndExtract_RespectsMaxArtifactSize(t *testing.T) {
	// Synthesize a payload bigger than the cap but otherwise well-formed.
	huge := bytes.Repeat([]byte{'x'}, MaxArtifactSize+1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(huge)
	}))
	defer srv.Close()

	_, _, err := FetchAndExtract(context.Background(), ArtifactRef{URL: srv.URL})
	if err == nil {
		t.Fatal("expected size cap error")
	}
	if !strings.Contains(err.Error(), "byte cap") {
		t.Errorf("unexpected err: %v", err)
	}
}

func TestFetchAndExtract_HTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, nil)
	}))
	defer srv.Close()

	_, _, err := FetchAndExtract(context.Background(), ArtifactRef{URL: srv.URL})
	if err == nil {
		t.Fatal("expected 404 error")
	}
}

// buildTar packs the given path → content map into a gzipped tar
// blob suitable as a fake source-controller artifact.
func buildTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var gzBuf bytes.Buffer
	gz := gzip.NewWriter(&gzBuf)
	tw := tar.NewWriter(gz)
	for path, content := range files {
		hdr := &tar.Header{
			Name:     path,
			Mode:     0o644,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", path, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write content %s: %v", path, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return gzBuf.Bytes()
}

// Sanity-check helper isolation — the reader works without bytes.NewReader.
func TestByteReader(t *testing.T) {
	r := bytesReader([]byte("hello"))
	out := make([]byte, 8)
	n, err := r.Read(out)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 || string(out[:n]) != "hello" {
		t.Errorf("got n=%d data=%q", n, out[:n])
	}
	// Second read returns EOF.
	_, err = r.Read(out)
	if err == nil {
		t.Fatal("expected EOF on second read")
	}
}

// Marker so the test compiles even if other tests skip.
var _ = fmt.Sprintf
