package fbc

import (
	"strings"
	"testing"
)

const (
	channelCandidate = `{"schema":"olm.channel","name":"candidate","package":"kubevirt-hyperconverged","entries":[{"name":"kubevirt-hyperconverged-operator.v4.22.5"}]}`
	channelStable    = `{"schema":"olm.channel","name":"stable","package":"kubevirt-hyperconverged","entries":[{"name":"kubevirt-hyperconverged-operator.v4.22.0"},{"name":"kubevirt-hyperconverged-operator.v4.22.2"}]}`

	bundleStableWinner = `{"schema":"olm.bundle","name":"kubevirt-hyperconverged-operator.v4.22.2","package":"kubevirt-hyperconverged","relatedImages":[{"name":"unrelated","image":"registry.redhat.io/container-native-virtualization/hco-bundle-registry-rhel9@sha256:unrelated"},{"name":"tekton-tasks","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-create-datavolume-rhel9@sha256:winnerdigest"},{"name":"disk-virt","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-disk-virt-customize-rhel9@sha256:winnerdiskvirt"},{"name":"virtio-win","image":"registry.redhat.io/container-native-virtualization/virtio-win-rhel9@sha256:winnervirtiowin"}]}`
	bundleStableOlder  = `{"schema":"olm.bundle","name":"kubevirt-hyperconverged-operator.v4.22.0","package":"kubevirt-hyperconverged","relatedImages":[{"name":"tekton-tasks","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-create-datavolume-rhel9@sha256:olderdigest"},{"name":"disk-virt","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-disk-virt-customize-rhel9@sha256:olderdiskvirt"},{"name":"virtio-win","image":"registry.redhat.io/container-native-virtualization/virtio-win-rhel9@sha256:oldervirtiowin"}]}`
	bundleCandidate    = `{"schema":"olm.bundle","name":"kubevirt-hyperconverged-operator.v4.22.5","package":"kubevirt-hyperconverged","relatedImages":[{"name":"tekton-tasks","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-create-datavolume-rhel9@sha256:candidatedigest"},{"name":"disk-virt","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-disk-virt-customize-rhel9@sha256:candidatediskvirt"},{"name":"virtio-win","image":"registry.redhat.io/container-native-virtualization/virtio-win-rhel9@sha256:candidatevirtiowin"}]}`
)

func joinObjects(objs ...string) string {
	return strings.Join(objs, "\n")
}

func TestParseStableRelease_HappyPath(t *testing.T) {
	data := joinObjects(channelCandidate, channelStable, bundleStableWinner, bundleStableOlder, bundleCandidate)

	release, err := parseStableRelease(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.Version.Original() != "v4.22.2" {
		t.Errorf("expected stable version v4.22.2 (not the newer candidate v4.22.5), got %s", release.Version.Original())
	}
	if release.ImageDigests[TektonTasksImageName] != "sha256:winnerdigest" {
		t.Errorf("expected tekton-tasks digest sha256:winnerdigest, got %s", release.ImageDigests[TektonTasksImageName])
	}
	if release.ImageDigests[DiskVirtImageName] != "sha256:winnerdiskvirt" {
		t.Errorf("expected disk-virt digest sha256:winnerdiskvirt, got %s", release.ImageDigests[DiskVirtImageName])
	}
	if release.ImageDigests[VirtioWinImageName] != "sha256:winnervirtiowin" {
		t.Errorf("expected virtio-win digest sha256:winnervirtiowin, got %s", release.ImageDigests[VirtioWinImageName])
	}
}

func TestParseStableRelease_OrderIndependence(t *testing.T) {
	orderings := [][]string{
		{bundleStableWinner, bundleStableOlder, bundleCandidate, channelCandidate, channelStable},
		{channelCandidate, bundleStableWinner, channelStable, bundleCandidate, bundleStableOlder},
	}

	for i, objs := range orderings {
		data := joinObjects(objs...)
		release, err := parseStableRelease(strings.NewReader(data))
		if err != nil {
			t.Fatalf("ordering %d: unexpected error: %v", i, err)
		}
		if release.Version.Original() != "v4.22.2" {
			t.Errorf("ordering %d: expected v4.22.2, got %s", i, release.Version.Original())
		}
		if release.ImageDigests[TektonTasksImageName] != "sha256:winnerdigest" {
			t.Errorf("ordering %d: expected sha256:winnerdigest, got %s", i, release.ImageDigests[TektonTasksImageName])
		}
	}
}

func TestParseStableRelease_NoStableChannel(t *testing.T) {
	data := joinObjects(channelCandidate, bundleCandidate)

	_, err := parseStableRelease(strings.NewReader(data))
	if err == nil {
		t.Fatal("expected error when no stable channel entries are present, got nil")
	}
}

func TestParseStableRelease_EmptyStableChannel(t *testing.T) {
	emptyStable := `{"schema":"olm.channel","name":"stable","package":"kubevirt-hyperconverged","entries":[]}`
	data := joinObjects(channelCandidate, emptyStable, bundleCandidate)

	_, err := parseStableRelease(strings.NewReader(data))
	if err == nil {
		t.Fatal("expected error when stable channel has zero entries, got nil")
	}
}

func TestParseStableRelease_NoMatchingRelatedImage(t *testing.T) {
	bundleNoMatch := `{"schema":"olm.bundle","name":"kubevirt-hyperconverged-operator.v4.22.2","package":"kubevirt-hyperconverged","relatedImages":[{"name":"unrelated","image":"registry.redhat.io/container-native-virtualization/hco-bundle-registry-rhel9@sha256:unrelated"}]}`
	data := joinObjects(channelStable, bundleNoMatch)

	_, err := parseStableRelease(strings.NewReader(data))
	if err == nil {
		t.Fatal("expected error when the winning bundle has no matching tracked images, got nil")
	}
}

func TestParseStableRelease_MissingTrackedImage(t *testing.T) {
	bundleMissing := `{"schema":"olm.bundle","name":"kubevirt-hyperconverged-operator.v4.22.2","package":"kubevirt-hyperconverged","relatedImages":[{"name":"tekton-tasks","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-create-datavolume-rhel9@sha256:winnerdigest"},{"name":"disk-virt","image":"registry.redhat.io/container-native-virtualization/kubevirt-tekton-tasks-disk-virt-customize-rhel9@sha256:winnerdiskvirt"}]}`
	data := joinObjects(channelStable, bundleMissing)

	_, err := parseStableRelease(strings.NewReader(data))
	if err == nil {
		t.Fatal("expected error when a tracked image is missing from the winning bundle, got nil")
	}
	if !strings.Contains(err.Error(), VirtioWinImageName) {
		t.Errorf("expected error to mention missing image %q, got: %v", VirtioWinImageName, err)
	}
}

func TestParseStableRelease_MalformedJSON(t *testing.T) {
	data := joinObjects(channelStable, bundleStableWinner) + "\n{not valid json"

	_, err := parseStableRelease(strings.NewReader(data))
	if err == nil {
		t.Fatal("expected error on malformed trailing JSON, got nil")
	}
}

func TestParseStableRelease_SkipsUnparseableVersion(t *testing.T) {
	badEntryChannel := `{"schema":"olm.channel","name":"stable","package":"kubevirt-hyperconverged","entries":[{"name":"kubevirt-hyperconverged-operator.not-a-version"},{"name":"kubevirt-hyperconverged-operator.v4.22.2"}]}`
	data := joinObjects(badEntryChannel, bundleStableWinner)

	release, err := parseStableRelease(strings.NewReader(data))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release.Version.Original() != "v4.22.2" {
		t.Errorf("expected unparseable entry to be skipped and v4.22.2 to win, got %s", release.Version.Original())
	}
}
