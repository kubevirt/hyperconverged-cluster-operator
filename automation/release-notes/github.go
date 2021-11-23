package main

import (
	"context"
	"fmt"
	"github.com/google/go-github/v32/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"log"
	"strings"
)

func (r *releaseData) gitHubInitClient() {
	tokenBytes, err := ioutil.ReadFile(r.githubTokenPath)
	if err != nil {
		log.Fatalf("ERROR accessing github token: %s ", err)
	}
	r.gitToken = strings.TrimSpace(string(tokenBytes))

	r.repoUrl = fmt.Sprintf("https://%s@github.com/%s/%s.git", r.gitToken, r.org, r.repo)
	r.infraUrl = fmt.Sprintf("https://%s@github.com/kubevirt/project-infra.git", r.gitToken)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: r.gitToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	r.githubClient = github.NewClient(tc)
}

func (r *releaseData) gitHubGetReleaseNote(number int) (string, error) {
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
		note, err := gitHubParseReleaseNote(i, line, body)
		if err == nil {
			note = fmt.Sprintf("[PR #%d][%s] %s", number, *pr.User.Login, note)
			return note, nil
		}
	}

	return "", nil
}

func gitHubParseReleaseNote(index int, line string, body []string) (string, error) {
	if strings.Contains(line, "```release-note") {
		releaseNoteIndex := index + 1
		if len(body) > releaseNoteIndex {
			note := strings.TrimSpace(body[releaseNoteIndex])
			// best effort at fixing some format errors I find
			note = strings.ReplaceAll(note, "\r\n", "")
			note = strings.ReplaceAll(note, "\r", "")
			note = strings.TrimPrefix(note, "- ")
			note = strings.TrimPrefix(note, "-")

			// best effort at catching "none" if the label didn't catch it
			if !strings.Contains(note, "NONE") && strings.ToLower(note) != "none" {
				return note, nil
			}
		}
	}

	return "", fmt.Errorf("release note not found")
}

func (r *releaseData) gitHubGetBranches() ([]*github.Branch, error) {
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

func (r *releaseData) gitHubGetReleases() ([]*github.RepositoryRelease, error) {
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
