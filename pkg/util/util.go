package util

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Options struct {
	DryRun         bool
	ExistingTags   string
	RepositoryURL  string
	MinimalVersion string
	Username       string
	GitToken       string
}

func LoadUpstreamSources() (map[string]string, error) {
	file, err := os.Open("upstream_sources.yaml")
	if err != nil {
		return nil, err
	}
	fscanner := bufio.NewScanner(file)
	upstreamSourcesMapping := make(map[string]string)
	for fscanner.Scan() {
		line := strings.Split(fscanner.Text(), ":")
		upstreamSourcesMapping[line[0]] = line[1]
	}
	return upstreamSourcesMapping, nil
}

func GetTektonTasksBranch(upstreamSources map[string]string, version string) (string, error) {
	var branch string
	var ok bool
	if branch, ok = upstreamSources[version]; !ok {
		return "", fmt.Errorf("There is missing mapping between branch of kubevirt-tekton-tasks and OCP V version " + version)
	}
	return branch, nil
}
