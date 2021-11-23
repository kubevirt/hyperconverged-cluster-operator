package main

import "github.com/google/go-github/v32/github"

type releaseData struct {
	repoDir   string
	infraDir  string
	cacheDir  string
	repoUrl   string
	infraUrl  string
	repo      string
	org       string
	newBranch string

	tagBranch   string
	tag         string
	previousTag string

	gitToken        string
	githubClient    *github.Client
	githubTokenPath string

	// github cached results
	allReleases []*github.RepositoryRelease
	allBranches []*github.Branch
}
