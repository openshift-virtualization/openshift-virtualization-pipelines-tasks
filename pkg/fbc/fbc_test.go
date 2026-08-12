package fbc

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

// fakeCatalogTree serves canned catalog.json contents (or errors) by path,
// standing in for a real git tree in tests of the stream-resolution loop.
type fakeCatalogTree struct {
	contentByPath map[string]string
	errByPath     map[string]error
}

func (f *fakeCatalogTree) openCatalogFile(path string) (io.ReadCloser, error) {
	if err, ok := f.errByPath[path]; ok {
		return nil, err
	}
	content, ok := f.contentByPath[path]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", path)
	}
	return io.NopCloser(strings.NewReader(content)), nil
}

func catalogPath(stream string) string {
	return fmt.Sprintf(catalogPathFmt, stream)
}

func TestResolveStreamReleases_MixOfHitsAndMisses(t *testing.T) {
	tree := &fakeCatalogTree{
		contentByPath: map[string]string{
			catalogPath("4.22"): joinObjects(channelStable, bundleStableWinner, bundleStableOlder),
		},
		errByPath: map[string]error{
			catalogPath("4.23"): errors.New("not found in tree"),
		},
	}

	results, err := resolveStreamReleases([]string{"4.22", "4.23"}, tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly one resolved stream, got %d: %v", len(results), results)
	}
	release, ok := results["4.22"]
	if !ok {
		t.Fatalf("expected stream 4.22 to resolve, got %v", results)
	}
	if release.Version.Original() != "v4.22.2" {
		t.Errorf("expected v4.22.2, got %s", release.Version.Original())
	}
	if _, ok := results["4.23"]; ok {
		t.Errorf("expected stream 4.23 to be skipped, not resolved")
	}
}

func TestResolveStreamReleases_ParseFailureIsSkipped(t *testing.T) {
	tree := &fakeCatalogTree{
		contentByPath: map[string]string{
			catalogPath("4.22"): joinObjects(channelStable, bundleStableWinner),
			catalogPath("4.23"): "{not valid json",
		},
	}

	results, err := resolveStreamReleases([]string{"4.22", "4.23"}, tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly one resolved stream, got %d: %v", len(results), results)
	}
	if _, ok := results["4.23"]; ok {
		t.Errorf("expected stream 4.23 to be skipped due to a parse failure")
	}
}

func TestResolveStreamReleases_AllStreamsFailReturnsError(t *testing.T) {
	tree := &fakeCatalogTree{
		errByPath: map[string]error{
			catalogPath("4.22"): errors.New("not found in tree"),
			catalogPath("4.23"): errors.New("not found in tree"),
		},
	}

	results, err := resolveStreamReleases([]string{"4.22", "4.23"}, tree)
	if err == nil {
		t.Fatal("expected an error when every stream fails to resolve, got nil")
	}
	if results != nil {
		t.Errorf("expected nil results alongside the error, got %v", results)
	}
}

func TestResolveStreamReleases_NoStreamsRequestedReturnsError(t *testing.T) {
	tree := &fakeCatalogTree{}

	_, err := resolveStreamReleases(nil, tree)
	if err == nil {
		t.Fatal("expected an error when no streams are requested, got nil")
	}
}

func TestResolveStreamReleases_CrossStreamVersionIsSkipped(t *testing.T) {
	tree := &fakeCatalogTree{
		contentByPath: map[string]string{
			catalogPath("4.22"): joinObjects(channelStable, bundleStableWinner, bundleStableOlder),
			catalogPath("4.23"): joinObjects(channelStable, bundleStableWinner, bundleStableOlder),
		},
	}

	results, err := resolveStreamReleases([]string{"4.22", "4.23"}, tree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := results["4.22"]; !ok {
		t.Error("expected stream 4.22 to resolve (version matches stream)")
	}
	if _, ok := results["4.23"]; ok {
		t.Error("expected stream 4.23 to be skipped because its highest stable version (v4.22.2) does not belong to the 4.23 stream")
	}
}
