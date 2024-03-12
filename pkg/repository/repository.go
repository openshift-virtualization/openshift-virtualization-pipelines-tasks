package repository

import (
	"os"

	"github.com/Masterminds/semver/v3"
	gh "github.com/cli/go-gh/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/util"
)

func CreateRelease(tag *semver.Version) error {
	generalNotes := "Release of OpenShift Virtualization Tekton tasks"
	args := []string{"release", "create", tag.String(), "kubevirt-tekton-tasks/release/catalog-cd/resources.tar.gz",
		"kubevirt-tekton-tasks/release/catalog-cd/catalog.yaml", "-R", "openshift-cnv/openshift-virtualization-pipelines-tasks", "-t", tag.String(),
		"-n", generalNotes}
	_, _, err := gh.Exec(args...)

	return err
}

func GetNewTags(oCPVTags, pTTags []*semver.Version) map[string]*semver.Version {
	newTags := map[string]*semver.Version{}
	for _, oCPVTag := range oCPVTags {
		found := false
		for _, pTTag := range pTTags {
			if oCPVTag.Equal(pTTag) {
				found = true
			}
		}
		if !found {
			newTags[oCPVTag.String()] = oCPVTag
		}
	}
	return newTags
}

func GetRepository(options *util.Options) (*git.Repository, error) {
	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		Auth: &http.BasicAuth{
			Username: options.Username,
			Password: options.GitToken,
		},
		URL:      options.RepositoryURL,
		Progress: os.Stdout,
	})

	if err != nil {
		return nil, err
	}
	return repo, nil
}
