package release

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/repository"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/util"
)

func ProcessNewReleases(options *util.Options) {
	minimalVersionConstraint, err := semver.NewConstraint(">= " + options.MinimalVersion)
	if err != nil {
		os.Exit(1)
	}
	filteredOCPVTags, err := filterOldOCPVTags(minimalVersionConstraint, options.ExistingTags)

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
				log.Println(version)
			}
		} else {
			err := createNewReleases(newTags)
			if err != nil {
				log.Fatal("something happened while creating new release: " + err.Error())
			}
		}
	} else {
		log.Println("nothing to do")
	}
}

func createNewReleases(newTags map[string]*semver.Version) error {
	mapping, err := util.LoadUpstreamSources()
	if err != nil {
		log.Fatal("err during loading upstream sources: " + err.Error())
	}

	for _, tag := range newTags {
		tektonTaskBranch, err := util.GetTektonTasksBranch(mapping, fmt.Sprintf("%v.%v", tag.Major(), tag.Minor()))
		if err != nil {
			return err
		}

		tag = semver.MustParse("v0.0.1-alpha.1") //replace tag with a testing tag - remove in the future
		tektonTaskBranch = "main"                //hardcode branch - remove in the future
		err = generateManifests(tag.String(), tektonTaskBranch)
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

func generateManifests(tag, branch string) error {
	os.Setenv("RELEASE_VERSION", tag)
	os.Setenv("RELEASE_BRANCH", branch)
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

func filterOldOCPVTags(minimalVersionConstraint *semver.Constraints, existingTagsStr string) ([]*semver.Version, error) {
	tags := strings.Split(existingTagsStr, ",")
	highestPatchOfMinorMap := map[string]*semver.Version{}
	existingTags := make([]*semver.Version, 0)
	for _, tag := range tags {
		version, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}
		if version.Prerelease() != "" {
			continue
		}

		majorMinorVersion := fmt.Sprintf("%v.%v", version.Major(), version.Minor())
		if highestVersion, ok := highestPatchOfMinorMap[majorMinorVersion]; ok {
			if highestVersion.Patch() < version.Minor() {
				highestPatchOfMinorMap[majorMinorVersion] = version
			}
		} else {
			highestPatchOfMinorMap[majorMinorVersion] = version
		}
	}
	for _, version := range highestPatchOfMinorMap {
		if minimalVersionConstraint.Check(version) {
			existingTags = append(existingTags, version)
		}
	}
	return existingTags, nil
}
