package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type releaseData struct {
	repoDir   string
	infraDir  string
	cacheDir  string
	repoUrl   string
	infraUrl  string
	repo      string
	org       string
	newBranch string

	tagBranch        string
	tag              string
	previousTag      string

	gitToken        string
	githubClient    *github.Client
	githubTokenPath string

	// github cached results
	allReleases []*github.RepositoryRelease
	allBranches []*github.Branch
}

func gitCommand(arg ...string) (string, error) {
	log.Printf("executing 'git %v", arg)
	cmd := exec.Command("git", arg...)
	bytes, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("ERROR: git command output: %s : %s ", string(bytes), err)
		return "", err
	}
	return string(bytes), nil
}

func (r *releaseData) checkoutUpstream() error {
	_, err := os.Stat(r.repoDir)
	if err == nil {
		_, err := gitCommand("-C", r.repoDir, "status")
		if err == nil {
			// checkout already exists. default to checkout main
			_, err = gitCommand("-C", r.repoDir, "checkout", "main")
			if err != nil {
				return err
			}

			_, err = gitCommand("-C", r.repoDir, "pull")
			if err != nil {
				return err
			}
			return nil
		}
	}

	// start fresh because checkout doesn't exist or is corrupted
	os.RemoveAll(r.repoDir)
	err = os.MkdirAll(r.repoDir, 0755)
	if err != nil {
		return err
	}

	// add upstream remote branch
	_, err = gitCommand("clone", r.repoUrl, r.repoDir)
	if err != nil {
		return err
	}

	_, err = gitCommand("-C", r.repoDir, "config", "diff.renameLimit", "999999")
	if err != nil {
		return err
	}

	return nil
}

func (r *releaseData) verifyTag() error {
	// must be a valid semver version
	tagSemver, err := semver.NewVersion(r.tag)
	if err != nil {
		return err
	}

	expectedBranch := fmt.Sprintf("release-%d.%d", tagSemver.Major(), tagSemver.Minor())

	releases, err := r.getReleases()
	for _, release := range releases {
		if *release.TagName == r.tag {
			log.Printf("WARNING: Release tag [%s] already exists", r.tag)
		}
	}

	var vs []*semver.Version

	for _, release := range releases {
		if (release.Draft != nil && *release.Draft) ||
			(release.Prerelease != nil && *release.Prerelease) {

			continue
		}
		v, err := semver.NewVersion(*release.TagName)
		if err != nil {
			// not an official release if it's not semver compatiable.
			continue
		}
		vs = append(vs, v)
	}

	// decending order from most recent.
	sort.Sort(sort.Reverse(semver.Collection(vs)))

	for _, v := range vs {
		if v.LessThan(tagSemver) {
			r.previousTag = fmt.Sprintf("v%v", v)
			break
		}
	}

	if r.previousTag == "" {
		log.Printf("No previous release tag found for tag [%s]", r.tag)
	} else {
		log.Printf("Previous Tag [%s]", r.previousTag)
	}

	branches, err := r.getBranches()
	if err != nil {
		return err
	}

	var releaseBranch *github.Branch
	for _, branch := range branches {
		if branch.Name != nil && *branch.Name == expectedBranch {
			releaseBranch = branch
			break
		}
	}

	if releaseBranch == nil {
		return fmt.Errorf("release branch [%s] not found for new release [%s]", expectedBranch, r.tag)
	}

	r.tagBranch = expectedBranch
	return nil
}

func (r *releaseData) getReleases() ([]*github.RepositoryRelease, error) {

	if len(r.allReleases) != 0 {
		return r.allReleases, nil
	}

	releases, _, err := r.githubClient.Repositories.ListReleases(context.Background(), r.org, r.repo, &github.ListOptions{PerPage: 10000})

	if err != nil {
		return nil, err
	}
	r.allReleases = releases

	return r.allReleases, nil
}

func (r *releaseData) getBranches() ([]*github.Branch, error) {

	if len(r.allBranches) != 0 {
		return r.allBranches, nil
	}

	branches, _, err := r.githubClient.Repositories.ListBranches(context.Background(), r.org, r.repo, &github.BranchListOptions{
		ListOptions: github.ListOptions{
			PerPage: 10000,
		},
	})
	if err != nil {
		return nil, err
	}
	r.allBranches = branches

	return r.allBranches, nil

}

func (r *releaseData) generateReleaseNotes() error {
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

	tagUrl := fmt.Sprintf("https://github.com/%s/%s/releases/tag/%s", r.org, r.repo, r.tag)

	releaseNotesFile := fmt.Sprintf("%s-release-notes.txt", r.tag)

	f, err := os.Create(releaseNotesFile)
	if err != nil {
		return err
	}
	defer f.Close()

	span := fmt.Sprintf("%s..origin/%s", r.previousTag, r.tagBranch)

	fullLogStr, err := gitCommand("-C", r.repoDir, "log", "--oneline", span)
	if err != nil {
		return err
	}

	var releaseNotes []string

	fullLogLines := strings.Split(fullLogStr, "\n")
	pattern := regexp.MustCompile(`\(#\d+\)`)

	for _, line := range fullLogLines {
		matches := pattern.FindAllString(line, -1)

		if len(matches) > 0 {
			for _, match := range matches {
				num, err := strconv.Atoi(match[2 : len(match)-1])
				if err != nil {
					continue
				}
				note, err := r.getReleaseNote(num)
				if err != nil {
					continue
				}
				if note != "" {
					releaseNotes = append(releaseNotes, note)
				}
			}
		}
	}

	logStr, err := gitCommand("-C", r.repoDir, "log", "--oneline", span)
	if err != nil {
		return err
	}

	contributorStr, err := gitCommand("-C", r.repoDir, "shortlog", "-sne", span)
	if err != nil {
		return err
	}

	contributorList := strings.Split(contributorStr, "\n")

	typeOfChanges, err := gitCommand("-C", r.repoDir, "diff", "--shortstat", span)
	if err != nil {
		return err
	}

	numChanges := strings.Count(logStr, "\n")
	numContributors := len(contributorList)
	typeOfChanges = strings.TrimSpace(typeOfChanges)

	f.WriteString(fmt.Sprintf("This release follows %s and consists of %d changes, contributed by %d people, leading to %s.\n", r.previousTag, numChanges, numContributors, typeOfChanges))
	f.WriteString("\n")
	f.WriteString(fmt.Sprintf("The source code and selected binaries are available for download at: %s.\n", tagUrl))
	f.WriteString("\n")
	f.WriteString("The primary release artifact of KubeVirt is the git tree. The release tag is\n")
	f.WriteString(fmt.Sprintf("signed and can be verified using `git tag -v %s`.\n", r.tag))
	f.WriteString("\n")
	f.WriteString(fmt.Sprintf("Pre-built containers are published on Quay and can be viewed at: <https://quay.io/%s/>.\n", r.org))
	f.WriteString("\n")

	if len(releaseNotes) > 0 {
		f.WriteString("Notable changes\n---------------\n")
		f.WriteString("\n")
		for _, note := range releaseNotes {
			f.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}

	f.WriteString("\n")
	f.WriteString("Contributors\n------------\n")
	f.WriteString(fmt.Sprintf("%d people contributed to this release:\n\n", numContributors))

	for _, contributor := range contributorList {
		if strings.Contains(contributor, "kubevirt-bot") || strings.Contains(contributor, "hco-bot") {
			// skip the bot
			continue
		}
		f.WriteString(fmt.Sprintf("%s\n", strings.TrimSpace(contributor)))
	}

	f.WriteString(additionalResources)
	return nil
}

func (r *releaseData) getReleaseNote(number int) (string, error) {

	log.Printf("Searching for release note for PR #%d", number)
	pr, _, err := r.githubClient.PullRequests.Get(context.Background(), r.org, r.repo, number)
	if err != nil {
		return "", err
	}

	for _, label := range pr.Labels {
		if label.Name != nil && *label.Name == "release-note-none" {
			return "", nil
		}
	}

	if pr.Body == nil || *pr.Body == "" {
		return "", err
	}

	body := strings.Split(*pr.Body, "\n")

	for i, line := range body {
		if strings.Contains(line, "```release-note") {
			releaseNoteIndex := i + 1
			if len(body) > releaseNoteIndex {
				note := strings.TrimSpace(body[releaseNoteIndex])
				// best effort at fixing some format errors I find
				note = strings.ReplaceAll(note, "\r\n", "")
				note = strings.ReplaceAll(note, "\r", "")
				note = strings.TrimPrefix(note, "- ")
				note = strings.TrimPrefix(note, "-")
				// best effort at catching "none" if the label didn't catch it
				if !strings.Contains(note, "NONE") && strings.ToLower(note) != "none" {
					note = fmt.Sprintf("[PR #%d][%s] %s", number, *pr.User.Login, note)
					return note, nil
				}
			}
		}
	}
	return "", nil
}

func main() {
	newBranch := flag.String("new-branch", "", "New branch to cut from main.")
	releaseTag := flag.String("new-release", "", "New release tag. Must be a valid semver. The branch is automatically detected from the major and minor release")
	org := flag.String("org", "", "The project org")
	repo := flag.String("repo", "", "The project repo")
	cacheDir := flag.String("cache-dir", "/tmp/release-tool", "The base directory used to cache git repos in")
	cleanCacheDir := flag.Bool("clean-cache", true, "Clean the cache dir before executing")
	githubTokenFile := flag.String("github-token-file", "", "file containing the github token.")

	flag.Parse()

	if *org == "" {
		log.Fatal("--org is a required argument")
	} else if *repo == "" {
		log.Fatal("--repo is a required argument")
	} else if *githubTokenFile == "" {
		log.Fatal("--github-token-file is a required argument")
	}

	tokenBytes, err := ioutil.ReadFile(*githubTokenFile)
	if err != nil {
		log.Fatalf("ERROR accessing github token: %s ", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	repoUrl := fmt.Sprintf("https://%s@github.com/%s/%s.git", token, *org, *repo)
	infraUrl := fmt.Sprintf("https://%s@github.com/kubevirt/project-infra.git", token)
	repoDir := fmt.Sprintf("%s/%s/https-%s", *cacheDir, *org, *repo)
	infraDir := fmt.Sprintf("%s/%s/https-%s", *cacheDir, "kubevirt", "project-infra")

	if *cleanCacheDir {
		os.RemoveAll(repoDir)
		os.RemoveAll(infraDir)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	r := releaseData{
		repoDir:   repoDir,
		infraDir:  infraDir,
		repoUrl:   repoUrl,
		infraUrl:  infraUrl,
		repo:      *repo,
		org:       *org,
		newBranch: *newBranch,
		tag:       *releaseTag,

		gitToken:        token,
		githubClient:    client,
		githubTokenPath: *githubTokenFile,
	}

	err = r.checkoutUpstream()
	if err != nil {
		log.Fatalf("ERROR checking out upstream: %s\n", err)
	}

	err = r.verifyTag()
	if err != nil {
		log.Fatalf("ERROR generating release notes: %s\n", err)
	}

	err = r.generateReleaseNotes()
	if err != nil {
		log.Fatalf("ERROR generating release notes: %s\n", err)
	}
}
