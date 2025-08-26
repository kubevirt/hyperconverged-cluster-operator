package util

import (
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/ghodss/yaml"
)

func UnmarshalYamlFileToObject(file io.Reader, o interface{}) error {
	yamlBytes, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(yamlBytes, o)
}

// ValidateManifestDir checks a directory contains manifests file in YAML format
// This function returns 3-state error:
//
//	err := ValidateManifestDir(...)
//	err == nil - OK: directory exists
//	err != nil && errors.Unwrap(err) == nil - directory does not exist, but that ok
//	err != nil && errors.Unwrap(err) != nil - actual error
func ValidateManifestDir(dirName string, dir fs.FS) error {
	// check if the directory exists
	info, err := fs.Stat(dir, dirName)
	if err != nil {
		if os.IsNotExist(err) { // don't return error if there is no such a dir, just ignore it
			return NewProcessingError(nil) // return error, but don't stop processing
		}
		return NewProcessingError(err)
	}

	if !info.IsDir() {
		err = fmt.Errorf("%s is not a directory", dirName)
		return NewProcessingError(err) // return error
	}

	return nil
}

func GetManifestDirPath(envVarName string, defaultDir string) string {
	return cmp.Or(os.Getenv(envVarName), defaultDir)
}
