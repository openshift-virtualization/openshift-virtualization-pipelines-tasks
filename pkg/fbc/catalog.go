package fbc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/Masterminds/semver/v3"
)

const (
	stableChannelName = "stable"
	bundleNamePrefix  = "kubevirt-hyperconverged-operator."

	// TektonTasksImageName is the short (registry-less) name of the
	// kubevirt-tekton-tasks image whose digest we track. It is the single
	// source of truth for this name: callers that need the full image
	// reference (e.g. generate-manifests.sh, via the RELEASE_IMAGE_NAME env
	// var set in pkg/release) build it from this constant rather than
	// hardcoding it again.
	TektonTasksImageName = "kubevirt-tekton-tasks-create-datavolume-rhel9"
)

// fbcObject is a superset of the fields we need across the different FBC
// schema variants ("olm.package", "olm.channel", "olm.bundle") that appear,
// concatenated one after another, in a catalog.json stream. Fields absent
// from a given object are simply left at their zero value.
type fbcObject struct {
	Schema  string `json:"schema"`
	Name    string `json:"name"`
	Entries []struct {
		Name string `json:"name"`
	} `json:"entries"`
	RelatedImages []struct {
		Image string `json:"image"`
	} `json:"relatedImages"`
}

// parseStableRelease reads a catalog.json stream (a sequence of concatenated
// JSON objects, not a single JSON document) and returns the highest version
// found in the "stable" channel along with the digest of the
// kubevirt-tekton-tasks-create-datavolume-rhel9 image used by that version's
// bundle.
func parseStableRelease(r io.Reader) (*StreamRelease, error) {
	dec := json.NewDecoder(r)

	var stableEntryNames []string
	bundleDigests := make(map[string]string)

	for {
		var obj fbcObject
		if err := dec.Decode(&obj); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decoding catalog.json object: %w", err)
		}

		switch obj.Schema {
		case "olm.channel":
			if obj.Name == stableChannelName {
				for _, e := range obj.Entries {
					stableEntryNames = append(stableEntryNames, e.Name)
				}
			}
		case "olm.bundle":
			for _, ri := range obj.RelatedImages {
				if strings.Contains(ri.Image, TektonTasksImageName) {
					if digest := digestFromImageRef(ri.Image); digest != "" {
						bundleDigests[obj.Name] = digest
					}
					break
				}
			}
		}
	}

	var highest *semver.Version
	var highestBundleName string
	for _, name := range stableEntryNames {
		v, err := semver.NewVersion(strings.TrimPrefix(name, bundleNamePrefix))
		if err != nil {
			continue
		}
		if highest == nil || v.GreaterThan(highest) {
			highest = v
			highestBundleName = name
		}
	}
	if highest == nil {
		return nil, fmt.Errorf("no %q channel entries found", stableChannelName)
	}

	digest, ok := bundleDigests[highestBundleName]
	if !ok {
		return nil, fmt.Errorf("no related image digest for bundle %q matching %q", highestBundleName, TektonTasksImageName)
	}

	return &StreamRelease{Version: highest, ImageDigest: digest}, nil
}

func digestFromImageRef(imageRef string) string {
	_, digest, found := strings.Cut(imageRef, "@")
	if !found {
		return ""
	}
	return digest
}
