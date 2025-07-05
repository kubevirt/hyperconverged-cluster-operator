package main

import (
	"fmt"
	"strings"
)

type fileType interface {
	getFileNameSuffix() string
}

type csvFileType struct{}

func (ft *csvFileType) getFileNameSuffix() string {
	return ".clusterserviceversion.yaml"
}

type fileTypeMultiFiles struct {
	name     string
	numFiles int
}

func newFileTypeMultiFiles(name string) *fileTypeMultiFiles {
	ft := &fileTypeMultiFiles{
		name:     name,
		numFiles: 0,
	}

	return ft
}

func (ft *fileTypeMultiFiles) getFileNameSuffix() string {
	n := fmt.Sprintf("%02d.%s.yaml", ft.numFiles, ft.name)
	ft.numFiles++
	return n
}

type fileTypes struct {
	types map[string]fileType
}

func (fts fileTypes) getFileType(line string) fileType {
	t, exists := fts.types[line]

	if !exists {
		newTypeName := strings.ToLower(strings.TrimPrefix(line, kindLinePrefix))
		t = newFileTypeMultiFiles(newTypeName)
		fts.types[line] = t
	}

	return t
}
