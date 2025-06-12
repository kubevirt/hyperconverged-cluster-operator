package main

import (
	"bytes"
	"fmt"
	"os"
	"path"

	"github.com/ghodss/yaml"
)

type fileHandler struct {
	buff          *bytes.Buffer
	fileTypeIsSet bool
	operatorName  string
	fileName      string
	outputDir     string
	isNewFile     bool
	fileTypeName  string
}

func newFileHandler(operatorName string, outputDir string) *fileHandler {
	return &fileHandler{
		operatorName: operatorName,
		buff:         bytes.NewBuffer([]byte("---\n")),
		outputDir:    outputDir,
		isNewFile:    true,
	}
}

func (fh *fileHandler) getFileName() string {
	return fh.fileName
}

func (fh *fileHandler) writeLine(l string) {
	fh.buff.WriteString(l)
	fh.buff.WriteByte('\n')
	fh.isNewFile = false
}

var (
	unknownFileType = indexedFileType{name: "unknown"}
	crdFileType     = indexedFileType{name: "crd"}
)

func (fh *fileHandler) writeToFile() error {
	var fileName string

	switch fh.fileTypeName {
	case "":
		fileName = fh.operatorName + unknownFileType.getFileNameSuffix()
	case "customresourcedefinition":
		fileName = fh.operatorName + crdFileType.getFileNameSuffix()

	case "clusterserviceversion":
		fileName = fh.operatorName + csvExtension
	default:
		object := map[string]any{}
		if err := yaml.Unmarshal(fh.buff.Bytes(), &object); err != nil {
			return err
		}

		name := object["metadata"].(map[string]any)["name"]
		fileName = fmt.Sprintf("%s.%s.%s.yaml", fh.operatorName, fh.fileTypeName, name)

	}

	fh.fileName = path.Join(outputDir, fileName)

	file, err := os.OpenFile(fh.getFileName(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(fh.buff.Bytes())

	return err
}

func (fh *fileHandler) setFileType(ft string) {
	fh.fileTypeName = ft
	fh.fileTypeIsSet = true
}

func (fh *fileHandler) isEmpty() bool {
	return fh.isNewFile
}
