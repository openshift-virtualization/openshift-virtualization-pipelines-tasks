package release

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/fbc"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/repository"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/util"
)

func ProcessNewReleases(options *util.Options) {
	minimalVersionConstraint, err := semver.NewConstraint(">= " + options.MinimalVersion)
	if err != nil {
		os.Exit(1)
	}

	upstreamSourcesMapping, err := util.LoadUpstreamSources()
	if err != nil {
		log.Fatal("err during loading upstream sources: " + err.Error())
	}
	streams := make([]string, 0, len(upstreamSourcesMapping))
	for stream := range upstreamSourcesMapping {
		streams = append(streams, stream)
	}

	streamReleases, err := fbc.FetchLatestStableReleases(streams)
	if err != nil {
		log.Fatal("err during fetching cnv-fbc releases: " + err.Error())
	}
	filteredOCPVTags, digestsByVersion := filterStreamReleases(minimalVersionConstraint, streamReleases)


	repo, err := repository.GetRepository(options)
	if err != nil {
		os.Exit(1)
	}

	pipelinesTasksExistingTags, err := repo.Tags()
	if err != nil {
		log.Fatal("err during retrieving of github tags for OCPVPT: " + err.Error())
	}
	var filteredPipelinesTasksTags []*semver.Version
	filteredPipelinesTasksTags, err = filterOldPipelinesTasksTags(minimalVersionConstraint, pipelinesTasksExistingTags)

	newTags := repository.GetNewTags(filteredOCPVTags, filteredPipelinesTasksTags)
	if len(newTags) > 0 {
		if options.DryRun {
			log.Println("DRY RUN enabled - these new tags would be created:")
			for version, tag := range newTags {
				log.Printf("%s:", tag.Original())
				digests := digestsByVersion[version]
				for _, name := range fbc.TrackedImageNames {
					log.Printf("  %s: %s", name, digests[name])
				}
			}
		} else {
			err := createNewReleases(newTags, digestsByVersion, upstreamSourcesMapping)
			if err != nil {
				log.Fatal("something happened while creating new release: " + err.Error())
			}
		}
	} else {
		log.Println("nothing to do")
	}
}

func createNewReleases(newTags map[string]*semver.Version, digestsByVersion map[string]map[string]string, upstreamSourcesMapping map[string]string) error {
	for _, tag := range newTags {
		tektonTaskBranch, err := util.GetTektonTasksBranch(upstreamSourcesMapping, fmt.Sprintf("%v.%v", tag.Major(), tag.Minor()))
		if err != nil {
			return err
		}

		digests, ok := digestsByVersion[tag.Original()]
		if !ok || len(digests) == 0 {
			return fmt.Errorf("no image digests found for tag %s", tag.Original())
		}
		for _, name := range fbc.TrackedImageNames {
			if _, found := digests[name]; !found {
				return fmt.Errorf("tag %s is missing digest for tracked image %q", tag.Original(), name)
			}
		}

		err = generateManifests(tag.Original(), tektonTaskBranch, digests)
		if err != nil {
			log.Fatal("err during generation of manifests: " + err.Error())
		}

		err = repository.CreateRelease(tag)
		if err != nil {
			log.Fatal("err during creating of new release: " + err.Error())
		}
	}
	return nil
}

func generateManifests(tag, branch string, imageDigests map[string]string) error {
	os.Setenv("RELEASE_VERSION", tag)
	os.Setenv("RELEASE_BRANCH", branch)
	os.Setenv("TEKTON_TASKS_IMAGE_NAME", fbc.TektonTasksImageName)
	os.Setenv("TEKTON_TASKS_IMAGE_DIGEST", imageDigests[fbc.TektonTasksImageName])
	os.Setenv("DISK_VIRT_IMAGE_DIGEST", imageDigests[fbc.DiskVirtImageName])
	os.Setenv("VIRTIO_WIN_IMAGE_DIGEST", imageDigests[fbc.VirtioWinImageName])
	cmd := exec.Command("bash", "-c", "./generate-manifests.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	err := cmd.Start()
	if err != nil {
		return err
	}

	err = cmd.Wait()
	return err
}

func filterOldPipelinesTasksTags(minimalVersionConstraint *semver.Constraints, existingPTTags storer.ReferenceIter) ([]*semver.Version, error) {
	existingTags := make([]*semver.Version, 0)
	existingPTTags.ForEach(func(tag *plumbing.Reference) error {
		version, err := semver.NewVersion(tag.Name().Short())
		if err != nil {
			return nil
		}
		if minimalVersionConstraint.Check(version) {
			existingTags = append(existingTags, version)
		}
		return nil
	})
	return existingTags, nil
}

// filterStreamReleases applies the minimal-version constraint to the
// per-stream releases fetched from cnv-fbc, returning the surviving versions
// alongside a lookup of image digests by version string (tag.Original()).
func filterStreamReleases(minimalVersionConstraint *semver.Constraints, streamReleases map[string]*fbc.StreamRelease) ([]*semver.Version, map[string]map[string]string) {
	versions := make([]*semver.Version, 0, len(streamReleases))
	digestsByVersion := make(map[string]map[string]string, len(streamReleases))
	for _, sr := range streamReleases {
		if minimalVersionConstraint.Check(sr.Version) {
			versions = append(versions, sr.Version)
			digestsByVersion[sr.Version.Original()] = sr.ImageDigests
		}
	}
	return versions, digestsByVersion
}
