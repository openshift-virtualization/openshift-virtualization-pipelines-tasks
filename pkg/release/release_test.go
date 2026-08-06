package release

import (
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/fbc"
)

func TestFilterStreamReleases(t *testing.T) {
	constraint, err := semver.NewConstraint(">= v4.18")
	if err != nil {
		t.Fatalf("unexpected error building constraint: %v", err)
	}

	streamReleases := map[string]*fbc.StreamRelease{
		"4.17": {Version: mustVersion(t, "v4.17.9"), ImageDigests: map[string]string{fbc.TektonTasksImageName: "sha256:below-minimal"}},
		"4.20": {Version: mustVersion(t, "v4.20.7"), ImageDigests: map[string]string{fbc.TektonTasksImageName: "sha256:tt-4-20-7", fbc.DiskVirtImageName: "sha256:dv-4-20-7", fbc.VirtioWinImageName: "sha256:vw-4-20-7"}},
		"4.22": {Version: mustVersion(t, "v4.22.2"), ImageDigests: map[string]string{fbc.TektonTasksImageName: "sha256:tt-4-22-2", fbc.DiskVirtImageName: "sha256:dv-4-22-2", fbc.VirtioWinImageName: "sha256:vw-4-22-2"}},
	}

	versions, digestsByVersion := filterStreamReleases(constraint, streamReleases)

	if len(versions) != 2 {
		t.Fatalf("expected 2 versions to survive the >= v4.18 constraint, got %d: %v", len(versions), versions)
	}

	seen := make(map[string]bool)
	for _, v := range versions {
		seen[v.Original()] = true
	}
	if !seen["v4.20.7"] || !seen["v4.22.2"] {
		t.Errorf("expected v4.20.7 and v4.22.2 to survive, got %v", seen)
	}
	if seen["v4.17.9"] {
		t.Errorf("expected v4.17.9 to be filtered out by the minimal version constraint")
	}

	if digestsByVersion["v4.20.7"][fbc.TektonTasksImageName] != "sha256:tt-4-20-7" {
		t.Errorf("expected tekton-tasks digest sha256:tt-4-20-7 for v4.20.7, got %s", digestsByVersion["v4.20.7"][fbc.TektonTasksImageName])
	}
	if digestsByVersion["v4.22.2"][fbc.VirtioWinImageName] != "sha256:vw-4-22-2" {
		t.Errorf("expected virtio-win digest sha256:vw-4-22-2 for v4.22.2, got %s", digestsByVersion["v4.22.2"][fbc.VirtioWinImageName])
	}
	if _, ok := digestsByVersion["v4.17.9"]; ok {
		t.Errorf("did not expect a digest entry for the filtered-out v4.17.9")
	}
}

func TestCreateNewReleases_NoDigestsForTag(t *testing.T) {
	tag := mustVersion(t, "v4.22.2")
	newTags := map[string]*semver.Version{"v4.22.2": tag}
	upstream := map[string]string{"4.22": "release-v4.22"}
	digestsByVersion := map[string]map[string]string{}

	err := createNewReleases(newTags, digestsByVersion, upstream)
	if err == nil {
		t.Fatal("expected error when digestsByVersion has no entry for the tag, got nil")
	}
}

func TestCreateNewReleases_EmptyDigestsForTag(t *testing.T) {
	tag := mustVersion(t, "v4.22.2")
	newTags := map[string]*semver.Version{"v4.22.2": tag}
	upstream := map[string]string{"4.22": "release-v4.22"}
	digestsByVersion := map[string]map[string]string{
		"v4.22.2": {},
	}

	err := createNewReleases(newTags, digestsByVersion, upstream)
	if err == nil {
		t.Fatal("expected error when digests map is empty, got nil")
	}
}

func TestCreateNewReleases_MissingTrackedImage(t *testing.T) {
	tag := mustVersion(t, "v4.22.2")
	newTags := map[string]*semver.Version{"v4.22.2": tag}
	upstream := map[string]string{"4.22": "release-v4.22"}
	digestsByVersion := map[string]map[string]string{
		"v4.22.2": {
			fbc.TektonTasksImageName: "sha256:aaa",
			fbc.DiskVirtImageName:    "sha256:bbb",
		},
	}

	err := createNewReleases(newTags, digestsByVersion, upstream)
	if err == nil {
		t.Fatal("expected error when a tracked image is missing, got nil")
	}
	if !strings.Contains(err.Error(), fbc.VirtioWinImageName) {
		t.Errorf("expected error to mention missing image %q, got: %v", fbc.VirtioWinImageName, err)
	}
}

func mustVersion(t *testing.T, v string) *semver.Version {
	t.Helper()
	version, err := semver.NewVersion(v)
	if err != nil {
		t.Fatalf("unexpected error parsing version %s: %v", v, err)
	}
	return version
}
