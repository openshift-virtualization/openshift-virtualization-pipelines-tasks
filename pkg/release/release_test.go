package release

import (
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
		"4.17": {Version: mustVersion(t, "v4.17.9"), ImageDigest: "sha256:below-minimal"},
		"4.20": {Version: mustVersion(t, "v4.20.7"), ImageDigest: "sha256:v4-20-7"},
		"4.22": {Version: mustVersion(t, "v4.22.2"), ImageDigest: "sha256:v4-22-2"},
	}

	versions, digestByVersion := filterStreamReleases(constraint, streamReleases)

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

	if digestByVersion["v4.20.7"] != "sha256:v4-20-7" {
		t.Errorf("expected digest sha256:v4-20-7 for v4.20.7, got %s", digestByVersion["v4.20.7"])
	}
	if digestByVersion["v4.22.2"] != "sha256:v4-22-2" {
		t.Errorf("expected digest sha256:v4-22-2 for v4.22.2, got %s", digestByVersion["v4.22.2"])
	}
	if _, ok := digestByVersion["v4.17.9"]; ok {
		t.Errorf("did not expect a digest entry for the filtered-out v4.17.9")
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
