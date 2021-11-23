package main

import (
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

func (r *releaseData) gitCheckoutUpstream() error {
	_, err := os.Stat(r.repoDir)
	if err == nil {
		_, err := gitCommand("-C", r.repoDir, "status")
		if err == nil {
			// checkout already exists, updating
			return r.gitUpdateFromUpstream()
		}
	}

	return r.gitCloneUpstream()
}

func (r *releaseData) gitUpdateFromUpstream() error {
	_, err := gitCommand("-C", r.repoDir, "checkout", "main")
	if err != nil {
		return err
	}

	_, err = gitCommand("-C", r.repoDir, "pull")
	if err != nil {
		return err
	}
	return nil
}

func (r *releaseData) gitCloneUpstream() error {
	// start fresh because checkout doesn't exist or is corrupted
	os.RemoveAll(r.repoDir)
	err := os.MkdirAll(r.repoDir, 0755)
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

func (r *releaseData) gitGetContributors(span string) ([]string, error) {
	contributorStr, err := gitCommand("-C", r.repoDir, "shortlog", "-sne", span)
	if err != nil {
		return nil, err
	}

	return strings.Split(contributorStr, "\n"), nil
}

func (r *releaseData) gitGetReleaseNotes(span string) ([]string, error) {
	fullLogStr, err := gitCommand("-C", r.repoDir, "log", "--oneline", span)
	if err != nil {
		return nil, err
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
				note, err := r.gitHubGetReleaseNote(num)
				if err != nil {
					continue
				}
				if note != "" {
					releaseNotes = append(releaseNotes, note)
				}
			}
		}
	}

	return releaseNotes, nil
}

func (r *releaseData) gitGetNumChanges(span string) (int, error) {
	logStr, err := gitCommand("-C", r.repoDir, "log", "--oneline", span)
	if err != nil {
		return -1, err
	}

	return strings.Count(logStr, "\n"), nil
}

func (r *releaseData) gitGetTypeOfChanges(span string) (string, error) {
	typeOfChanges, err := gitCommand("-C", r.repoDir, "diff", "--shortstat", span)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(typeOfChanges), nil
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
