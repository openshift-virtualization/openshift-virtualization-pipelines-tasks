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

	TektonTasksImageName = "kubevirt-tekton-tasks-create-datavolume-rhel9"
	DiskVirtImageName    = "kubevirt-tekton-tasks-disk-virt-customize-rhel9"
	VirtioWinImageName   = "virtio-win-rhel9"
)

var TrackedImageNames = []string{
	TektonTasksImageName,
	DiskVirtImageName,
	VirtioWinImageName,
}

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
// found in the "stable" channel along with the digests of all tracked images
// used by that version's bundle.
func parseStableRelease(r io.Reader) (*StreamRelease, error) {
	dec := json.NewDecoder(r)

	var stableEntryNames []string
	// bundleDigests[bundleName][imageName] = digest
	bundleDigests := make(map[string]map[string]string)

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
				for _, tracked := range TrackedImageNames {
					if strings.Contains(ri.Image, tracked) {
						if digest := digestFromImageRef(ri.Image); digest != "" {
							if bundleDigests[obj.Name] == nil {
								bundleDigests[obj.Name] = make(map[string]string)
							}
							bundleDigests[obj.Name][tracked] = digest
						}
					}
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

	digests, ok := bundleDigests[highestBundleName]
	if !ok || len(digests) == 0 {
		return nil, fmt.Errorf("no related image digests for bundle %q", highestBundleName)
	}
	for _, name := range TrackedImageNames {
		if _, found := digests[name]; !found {
			return nil, fmt.Errorf("bundle %q is missing digest for tracked image %q", highestBundleName, name)
		}
	}

	return &StreamRelease{Version: highest, ImageDigests: digests}, nil
}

func digestFromImageRef(imageRef string) string {
	_, digest, found := strings.Cut(imageRef, "@")
	if !found {
		return ""
	}
	return digest
}
