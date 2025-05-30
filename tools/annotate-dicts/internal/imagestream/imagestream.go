package imagestream

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"

	imagev1 "github.com/openshift/api/image/v1"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"

	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/config"
)

// BuildImageStreamMap builds map of image stream name to the image url
func BuildImageStreamMap() (map[string]string, error) {
	entries, err := os.ReadDir(config.ImageStreamDir())
	if err != nil {
		return nil, fmt.Errorf("error reading image stream directory %s: %v", config.ImageStreamDir(), err)
	}

	isMap := make(map[string]string)
	group := errgroup.Group{}
	lock := &sync.Mutex{}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := path.Join(config.ImageStreamDir(), entry.Name())
		if ext := path.Ext(filename); ext != ".yaml" && ext != ".yml" {
			continue
		}

		log.Println("Reading ImageStream data from", filename)

		group.Go(getImageFromISFile(filename, lock, isMap))
	}

	err = group.Wait()
	if err != nil {
		return nil, fmt.Errorf("failed to build the image stream map: %v", err)
	}

	return isMap, nil
}

func readISFile(filename string) (*imagev1.ImageStream, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var is imagev1.ImageStream
	if err := yaml.Unmarshal(content, &is); err != nil {
		return nil, err
	}

	return &is, nil
}

func getImageFromISFile(filename string, lock *sync.Mutex, isMap map[string]string) func() error {
	return func() error {
		is, err := readISFile(filename)

		if err != nil {
			return fmt.Errorf("error reading image stream file %s: %v", filename, err)
		}
		if is == nil {
			return fmt.Errorf("can't parse image stream file %s", filename)
		}

		lock.Lock()
		defer lock.Unlock()

		if len(is.Spec.Tags) > 0 {
			if from := is.Spec.Tags[0].From; from != nil {
				isMap[is.Name] = from.Name
			}
		}

		return nil
	}
}
