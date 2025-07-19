package main

import (
	"fmt"
)

type indexedFileType struct {
	name     string
	numFiles int
}

func (ft *indexedFileType) getFileNameSuffix() string {
	n := fmt.Sprintf("%02d.%s.yaml", ft.numFiles, ft.name)
	ft.numFiles++
	return n
}
