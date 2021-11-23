package main

import (
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/google/go-github/v32/github"
	"log"
	"sort"
)

func semverGetVersions(releases []*github.RepositoryRelease) []*semver.Version {
	var vs []*semver.Version

	for _, release := range releases {
		if (release.Draft != nil && *release.Draft) ||
			(release.Prerelease != nil && *release.Prerelease) {

			continue
		}
		v, err := semver.NewVersion(*release.TagName)
		if err != nil {
			// not an official release if it's not semver compatible.
			continue
		}
		vs = append(vs, v)
	}

	// descending order from most recent.
	sort.Sort(sort.Reverse(semver.Collection(vs)))

	return vs
}

func (r *releaseData) semverCalculatePreviousRelease(tagSemver *semver.Version) error {
	releases, err := r.gitHubGetReleases()
	if err != nil {
		return err
	}

	for _, release := range releases {
		if *release.TagName == r.tag {
			log.Printf("WARNING: Release tag [%s] already exists", r.tag)
		}
	}

	vs := semverGetVersions(releases)
	for _, v := range vs {
		if v.LessThan(tagSemver) {
			r.previousTag = fmt.Sprintf("v%v", v)
			break
		}
	}

	return nil
}

func (r *releaseData) semverVerifyReleaseBranch(expectedBranch string) error {
	branches, err := r.gitHubGetBranches()
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

	return nil
}

func (r *releaseData) semverVerifyTag() error {
	// must be a valid semver version
	tagSemver, err := semver.NewVersion(r.tag)
	if err != nil {
		return err
	}

	expectedBranch := fmt.Sprintf("release-%d.%d", tagSemver.Major(), tagSemver.Minor())

	err = r.semverCalculatePreviousRelease(tagSemver)
	if err != nil {
		return err
	}

	if r.previousTag == "" {
		log.Printf("No previous release tag found for tag [%s]", r.tag)
	} else {
		log.Printf("Previous Tag [%s]", r.previousTag)
	}

	err = r.semverVerifyReleaseBranch(expectedBranch)
	if err != nil {
		return err
	}

	r.tagBranch = expectedBranch
	return nil
}
