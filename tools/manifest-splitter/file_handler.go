package main

import (
	"bytes"
	"os"
	"path"
)

type fileHandler struct {
	buff          *bytes.Buffer
	fileTypeIsSet bool
	ft            fileType
	operatorName  string
	fileName      string
	outputDir     string
}

func newFileHandler(operatorName string, defaultFT fileType, outputDir string) *fileHandler {
	return &fileHandler{
		operatorName: operatorName,
		ft:           defaultFT,
		buff:         &bytes.Buffer{},
		outputDir:    outputDir,
	}
}

func (fh *fileHandler) getFileName() string {
	if fh.fileName == "" {
		fh.fileName = fh.operatorName + fh.ft.getFileNameSuffix()
	}

	return path.Join(outputDir, fh.fileName)
}

func (fh *fileHandler) writeLine(l string) {
	fh.buff.WriteString(l)
	fh.buff.WriteByte('\n')
}

func (fh *fileHandler) writeToFile() error {
	return os.WriteFile(fh.getFileName(), fh.buff.Bytes(), 0644)
}

func (fh *fileHandler) setFileType(ft fileType) {
	fh.ft = ft
	fh.fileTypeIsSet = true
}

func (fh *fileHandler) isEmpty() bool {
	return fh.buff.Len() == 0
}
