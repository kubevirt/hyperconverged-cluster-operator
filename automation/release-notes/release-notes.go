package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func (r *releaseData) writeHeader(f *os.File, span string, contributorList []string) error {
	tagUrl := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", r.org, r.repo, r.tag)

	numChanges, err := r.gitGetNumChanges(span)
	if err != nil {
		return err
	}

	typeOfChanges, err := r.gitGetTypeOfChanges(span)
	if err != nil {
		return err
	}

	f.WriteString(fmt.Sprintf("This release follows %s and consists of %d changes, contributed by %d people, leading to %s.\n", r.previousTag, numChanges, len(contributorList), typeOfChanges))
	f.WriteString("\n")
	f.WriteString(fmt.Sprintf("The source code and selected binaries are available for download at: %s.\n", tagUrl))
	f.WriteString("\n")
	f.WriteString("The primary release artifact of KubeVirt is the git tree. The release tag is\n")
	f.WriteString(fmt.Sprintf("signed and can be verified using `git tag -v %s`.\n", r.tag))
	f.WriteString("\n")
	f.WriteString(fmt.Sprintf("Pre-built containers are published on Quay and can be viewed at: <https://quay.io/%s/>.\n", r.org))
	f.WriteString("\n")

	return nil
}

func (r *releaseData) writeNotableChanges(f *os.File, span string) error {
	releaseNotes, err := r.gitGetReleaseNotes(span)
	if err != nil {
		return err
	}

	if len(releaseNotes) > 0 {
		f.WriteString("Notable changes\n---------------\n")
		f.WriteString("\n")
		for _, note := range releaseNotes {
			f.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}

	return nil
}

func isBot(contributor string) bool {
	bots := []string{
		"kubevirt-bot",
		"hco-bot",
	}

	for _, bot := range bots {
		if strings.Contains(contributor, bot) {
			return true
		}
	}
	return false
}

func writeContributors(f *os.File, contributorList []string) {
	f.WriteString("\n")
	f.WriteString("Contributors\n------------\n")
	f.WriteString(fmt.Sprintf("%d people contributed to this release:\n\n", len(contributorList)))

	for _, contributor := range contributorList {
		if isBot(contributor) {
			// skip the bot
			continue
		}
		f.WriteString(fmt.Sprintf("%s\n", strings.TrimSpace(contributor)))
	}
}

func (r *releaseData) writeAdditionalResources(f *os.File) {
	additionalResources := fmt.Sprintf(`Additional Resources
--------------------
- Mailing list: <https://groups.google.com/forum/#!forum/kubevirt-dev>
- Slack: <https://kubernetes.slack.com/messages/virtualization>
- An easy to use demo: <https://github.com/%s/demo>
- [How to contribute][contributing]
- [License][license]


[contributing]: https://github.com/%s/%s/blob/main/CONTRIBUTING.md
[license]: https://github.com/%s/%s/blob/main/LICENSE
---
`, r.org, r.org, r.repo, r.org, r.repo)

	f.WriteString(additionalResources)
}

func (r *releaseData) generateReleaseNotes() error {
	releaseNotesFile := fmt.Sprintf("%s-release-notes.txt", r.tag)

	f, err := os.Create(releaseNotesFile)
	if err != nil {
		return err
	}
	defer f.Close()

	span := fmt.Sprintf("%s..origin/%s", r.previousTag, r.tagBranch)

	contributorList, err := r.gitGetContributors(span)
	if err != nil {
		return err
	}

	err = r.writeHeader(f, span, contributorList)
	if err != nil {
		return err
	}

	r.writeNotableChanges(f, span)
	writeContributors(f, contributorList)
	r.writeAdditionalResources(f)

	return nil
}

func parseArguments() releaseData {
	newBranch := flag.String("new-branch", "", "New branch to cut from main.")
	releaseTag := flag.String("new-release", "", "New release tag. Must be a valid semver. The branch is automatically detected from the major and minor release")
	org := flag.String("org", "", "The project org")
	repo := flag.String("repo", "", "The project repo")
	cacheDir := flag.String("cache-dir", "/tmp/release-tool", "The base directory used to cache git repos in")
	githubTokenFile := flag.String("github-token-file", "", "file containing the github token.")

	flag.Parse()

	if *org == "" {
		log.Fatal("--org is a required argument")
	} else if *repo == "" {
		log.Fatal("--repo is a required argument")
	} else if *githubTokenFile == "" {
		log.Fatal("--github-token-file is a required argument")
	}

	repoDir := fmt.Sprintf("%s/%s/https-%s", *cacheDir, *org, *repo)
	infraDir := fmt.Sprintf("%s/%s/https-%s", *cacheDir, "kubevirt", "project-infra")

	return releaseData{
		repoDir:   repoDir,
		infraDir:  infraDir,
		repo:      *repo,
		org:       *org,
		newBranch: *newBranch,
		tag:       *releaseTag,

		githubTokenPath: *githubTokenFile,
	}
}

func main() {
	r := parseArguments()

	r.gitHubInitClient()

	err := r.gitCheckoutUpstream()
	if err != nil {
		log.Fatalf("ERROR checking out upstream: %s\n", err)
	}

	err = r.semverVerifyTag()
	if err != nil {
		log.Fatalf("ERROR generating release notes: %s\n", err)
	}

	err = r.generateReleaseNotes()
	if err != nil {
		log.Fatalf("ERROR generating release notes: %s\n", err)
	}
}
