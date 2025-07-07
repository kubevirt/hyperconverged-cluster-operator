package handlers

import (
	"os"
	"path"
	"strings"

	. "github.com/onsi/gomega"
)

const (
	pkgDirectory = "controllers/handlers"
	testFilesLoc = "testFiles"
)

func getTestFilesLocation() string {
	wd, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())
	if strings.HasSuffix(wd, pkgDirectory) {
		return testFilesLoc
	}
	return path.Join(pkgDirectory, testFilesLoc)
}
