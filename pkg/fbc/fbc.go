package fbc

import (
	"fmt"
	"io"
	"log"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

const (
	fbcRepositoryURL = "https://github.com/openshift-cnv/cnv-fbc"
	// fbcBranch tracks what is actually live on registry.redhat.io. The
	// "stage" branch can carry pre-promotion catalog changes, which would
	// let us pick up a digest that hasn't shipped yet.
	fbcBranch      = "production"
	catalogPathFmt = "v%s/catalog/kubevirt-hyperconverged/catalog.json"
)

// StreamRelease describes the highest version available in the "stable"
// channel of a single OCP-V major.minor stream, and the pinned digest of the
// kubevirt-tekton-tasks image that ships with it.
type StreamRelease struct {
	Version     *semver.Version
	ImageDigest string
}

// catalogTree is the subset of a git tree needed to look up per-stream
// catalog files. It exists so the stream-resolution loop can be unit tested
// with a fake, without cloning a real repository.
type catalogTree interface {
	openCatalogFile(path string) (io.ReadCloser, error)
}

type gitCatalogTree struct {
	tree *object.Tree
}

func (t *gitCatalogTree) openCatalogFile(path string) (io.ReadCloser, error) {
	file, err := t.tree.File(path)
	if err != nil {
		return nil, err
	}
	return file.Reader()
}

// FetchLatestStableReleases clones the cnv-fbc catalog and, for each given
// major.minor stream (e.g. "4.22"), returns the highest version present in
// the "stable" channel along with the pinned digest of the
// kubevirt-tekton-tasks-create-datavolume-rhel9 image used by that version.
//
// Streams whose catalog can't be resolved (not yet published, malformed,
// missing image reference, ...) are logged and skipped rather than failing
// the whole run. An error is returned only if none of the streams resolved.
func FetchLatestStableReleases(streams []string) (map[string]*StreamRelease, error) {
	tree, err := cloneCatalogTree()
	if err != nil {
		return nil, fmt.Errorf("cloning %s (%s branch): %w", fbcRepositoryURL, fbcBranch, err)
	}
	return resolveStreamReleases(streams, &gitCatalogTree{tree: tree})
}

func resolveStreamReleases(streams []string, tree catalogTree) (map[string]*StreamRelease, error) {
	results := make(map[string]*StreamRelease, len(streams))
	for _, stream := range streams {
		path := fmt.Sprintf(catalogPathFmt, stream)

		reader, err := tree.openCatalogFile(path)
		if err != nil {
			log.Printf("fbc: stream %s: could not open catalog file %s: %v", stream, path, err)
			continue
		}

		release, err := parseStableRelease(reader)
		reader.Close()
		if err != nil {
			log.Printf("fbc: stream %s: failed to parse catalog %s: %v", stream, path, err)
			continue
		}

		results[stream] = release
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no releases could be resolved for any of the %d requested streams: %v", len(streams), streams)
	}
	return results, nil
}

func cloneCatalogTree() (*object.Tree, error) {
	repo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           fbcRepositoryURL,
		ReferenceName: plumbing.NewBranchReferenceName(fbcBranch),
		SingleBranch:  true,
		Depth:         1,
		Tags:          git.NoTags,
	})
	if err != nil {
		return nil, err
	}

	head, err := repo.Head()
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil, err
	}

	return commit.Tree()
}
