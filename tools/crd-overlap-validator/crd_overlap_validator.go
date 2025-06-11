package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ghodss/yaml"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	"k8s.io/apimachinery/pkg/util/sets"
)

func main() {
	crdDir := flag.String("crds-dir", "", "the directory containing the CRDs for apigroup validation. The validation will be performed if and only if the value is non-empty.")

	flag.Parse()

	if *crdDir == "" {
		fmt.Println("No CRD directory provided. Skipping API group overlap validation.")
		flag.Usage()
		os.Exit(1)
	}

	err := validateNoAPIOverlap(*crdDir)
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error())
		os.Exit(1)
	}

	fmt.Println("Validation succeeded: no overlapping API Groups found between different CRDs.")
}

func IOReadDir(root string) ([]string, error) {
	var files []string
	fileInfo, err := os.ReadDir(root)
	if err != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, filepath.Join(root, file.Name()))
	}
	return files, nil
}

func validateNoAPIOverlap(crdDir string) error {
	crdFiles, err := IOReadDir(crdDir)
	if err != nil {
		return err
	}

	// crdMap is populated with operator names as keys and a slice of associated api groups as values.
	crdMap, err := getCrdMap(crdFiles)
	if err != nil {
		return fmt.Errorf("failed to get CRD map: %v", err)
	}

	overlapsMap := detectAPIOverlap(crdMap)

	return checkAPIOverlapMap(overlapsMap)
}

func checkAPIOverlapMap(overlapsMap map[string]sets.Set[string]) error {
	// if at least one overlap found - emit an error.
	if len(overlapsMap) != 0 {
		var sb strings.Builder
		// WriteString always returns error=nil. no point to check it.
		_, _ = sb.WriteString("ERROR: Overlapping API Groups were found between different operators.\n")
		for apiGroup := range overlapsMap {
			_, _ = sb.WriteString(fmt.Sprintf("The API Group %s is being used by these operators: %s\n", apiGroup, strings.Join(overlapsMap[apiGroup].UnsortedList(), ", ")))
		}
		return errors.New(sb.String())
	}
	return nil
}

func detectAPIOverlap(crdMap map[string][]string) map[string]sets.Set[string] {
	// overlapsMap is populated with collisions found - API Groups as keys,
	// and slice containing operators using them, as values.
	overlapsMap := make(map[string]sets.Set[string])
	for operator, groups := range crdMap {
		for _, apiGroup := range groups {
			compareMapWithEntry(crdMap, operator, apiGroup, overlapsMap)
		}
	}
	return overlapsMap
}

func compareMapWithEntry(crdMap map[string][]string, operator string, apigroup string, overlapsMap map[string]sets.Set[string]) {
	for comparedOperator := range crdMap {
		if operator == comparedOperator { // don't check self
			continue
		}

		if slices.Contains(crdMap[comparedOperator], apigroup) {
			if overlapsMap[apigroup] == nil {
				overlapsMap[apigroup] = sets.New[string](operator, comparedOperator)
			} else {
				overlapsMap[apigroup].Insert(operator)
				overlapsMap[apigroup].Insert(comparedOperator)
			}
		}
	}
}

func getCrdMap(crdFiles []string) (map[string][]string, error) {
	crdMap := make(map[string][]string)

	for _, crdFilePath := range crdFiles {
		content, err := os.ReadFile(crdFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CRD file %s: %v", crdFilePath, err)
		}

		crdFileName := filepath.Base(crdFilePath)
		reg := regexp.MustCompile(`(\D+)`)
		operator := reg.FindString(crdFileName)

		var crd apiextensions.CustomResourceDefinition
		err = yaml.Unmarshal(content, &crd)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CRD file %s: %v", crdFilePath, err)
		}

		if !slices.Contains(crdMap[operator], crd.Spec.Group) {
			crdMap[operator] = append(crdMap[operator], crd.Spec.Group)
		}
	}

	return crdMap, nil
}
