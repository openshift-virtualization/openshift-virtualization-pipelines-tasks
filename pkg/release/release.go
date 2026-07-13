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
	filteredOCPVTags, digestByVersion := filterStreamReleases(minimalVersionConstraint, streamReleases)

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
			for version := range newTags {
				log.Printf("%s (image digest: %s)", version, digestByVersion[version])
			}
		} else {
			err := createNewReleases(newTags, digestByVersion, upstreamSourcesMapping)
			if err != nil {
				log.Fatal("something happened while creating new release: " + err.Error())
			}
		}
	} else {
		log.Println("nothing to do")
	}
}

func createNewReleases(newTags map[string]*semver.Version, digestByVersion map[string]string, upstreamSourcesMapping map[string]string) error {
	for _, tag := range newTags {
		tektonTaskBranch, err := util.GetTektonTasksBranch(upstreamSourcesMapping, fmt.Sprintf("%v.%v", tag.Major(), tag.Minor()))
		if err != nil {
			return err
		}

		digest, ok := digestByVersion[tag.Original()]
		if !ok || digest == "" {
			return fmt.Errorf("no image digest found for tag %s", tag.Original())
		}

		err = generateManifests(tag.Original(), tektonTaskBranch, digest)
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

func generateManifests(tag, branch, imageDigest string) error {
	os.Setenv("RELEASE_VERSION", tag)
	os.Setenv("RELEASE_BRANCH", branch)
	os.Setenv("RELEASE_IMAGE_DIGEST", imageDigest)
	os.Setenv("RELEASE_IMAGE_NAME", fbc.TektonTasksImageName)
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
// alongside a lookup of image digest by version string (tag.Original()).
func filterStreamReleases(minimalVersionConstraint *semver.Constraints, streamReleases map[string]*fbc.StreamRelease) ([]*semver.Version, map[string]string) {
	versions := make([]*semver.Version, 0, len(streamReleases))
	digestByVersion := make(map[string]string, len(streamReleases))
	for _, sr := range streamReleases {
		if minimalVersionConstraint.Check(sr.Version) {
			versions = append(versions, sr.Version)
			digestByVersion[sr.Version.Original()] = sr.ImageDigest
		}
	}
	return versions, digestByVersion
}
