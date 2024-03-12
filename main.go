package main

import (
	"log"
	"os"

	"github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/release"
	utils "github.com/openshift-cnv/openshift-virtualization-pipelines-tasks/pkg/util"
	"github.com/spf13/cobra"
)

func main() {
	options := &utils.Options{
		DryRun: true,
	}
	command := &cobra.Command{
		Use:   "OVPT",
		Short: "OVPT checks if new release of OCP V is available and if yes, creates a new tag and new release",
		Run: func(cmd *cobra.Command, args []string) {
			release.ProcessNewReleases(options)
		},
	}
	command.PersistentFlags().StringVar(&options.MinimalVersion, "minimal-version",
		"", "Do not check versions older than this, expected format: vx.y")
	command.PersistentFlags().StringVar(&options.ExistingTags, "existing-tags",
		"", "list of all existing container tags")
	command.PersistentFlags().StringVar(&options.RepositoryURL, "repository-url",
		"", "url of repository where to check releases")
	command.PersistentFlags().BoolVar(&options.DryRun, "dry-run",
		options.DryRun, "don't create anything")

	var set bool
	if options.GitToken, set = os.LookupEnv("ACTIONS_TOKEN"); !set {
		log.Fatal("Github token is not set")
	}
	if options.Username, set = os.LookupEnv("USERNAME"); !set {
		log.Fatal("Github username is not set")
	}
	if err := command.Execute(); err != nil {
		os.Exit(1)
	}
}
