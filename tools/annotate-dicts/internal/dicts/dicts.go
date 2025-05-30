package dicts

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/cleanup"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/annotate-dicts/internal/images"
)

const (
	MultiArchDICTAnnotation = "ssp.kubevirt.io/dict.architectures"
)

type Dicts struct {
	items   []hcov1beta1.DataImportCronTemplate
	lock    sync.Mutex
	changed bool
	group   *errgroup.Group
}

func NewDicts(group *errgroup.Group, filename string) (*Dicts, error) {
	log.Println("Reading DataImportCronTemplate file", filename)

	rawYaml, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading DataImportCronTemplate file %s: %v", filename, err)
	}

	var ds []hcov1beta1.DataImportCronTemplate
	if err := yaml.Unmarshal(rawYaml, &ds); err != nil {
		return nil, err
	}

	return &Dicts{
		items: ds,
		group: group,
	}, nil
}

func (d *Dicts) Run(ctx context.Context, isMap map[string]string) (bool, error) {
	log.Printf("annotating the DataImportCronTemplate objects with the supported architectures")
	for i := range d.items {
		d.handleOneDict(ctx, i, isMap)
	}

	if err := d.group.Wait(); err != nil {
		return false, err
	}

	return d.changed, nil
}

func (d *Dicts) handleOneDict(ctx context.Context, i int, isMap map[string]string) {
	dict := d.items[i]

	if dict.Spec.Template.Spec.Source == nil || dict.Spec.Template.Spec.Source.Registry == nil {
		return
	}

	log.Printf("Processing DataImportCronTemplate object %s", dict.Name)
	reg := dict.Spec.Template.Spec.Source.Registry

	var url string
	if reg.URL != nil {
		url = *reg.URL
	} else if reg.ImageStream != nil {
		if isURL, foundIS := isMap[*reg.ImageStream]; foundIS {
			url = isURL
		}
	}

	if url == "" {
		log.Printf("No image defined for DataImportCronTemplate object %s", dict.Name)
		return
	}

	d.group.Go(d.updateAnnotation(ctx, &dict, url))
}

func (d *Dicts) updateAnnotation(ctx context.Context, dict *hcov1beta1.DataImportCronTemplate, url string) func() error {
	return func() error {
		log.Printf("Reading the manifest for DataImportCronTemplate object %s; image: %s", dict.Name, url)
		arches, err := images.GetArches(ctx, url)
		if err != nil {
			return fmt.Errorf("error getting architecturs for %s: %v", url, err)
		}

		if arches == "" {
			log.Printf("The %s images is not a multi-architecture manifest", url)
			return nil
		}
		log.Printf("Found the following architectures in %q: %q", url, arches)

		origArches, ok := dict.Annotations[MultiArchDICTAnnotation]
		if ok && arches == origArches {
			// no change for this dict
			return nil
		}

		log.Printf("Annotating the DataImportCronTemplate object %s with %s=%q",
			dict.Name, MultiArchDICTAnnotation,
			arches)

		d.lock.Lock()
		defer d.lock.Unlock()

		d.changed = true
		if dict.Annotations == nil {
			dict.Annotations = make(map[string]string)
		}
		dict.Annotations[MultiArchDICTAnnotation] = arches

		return nil
	}
}

func (d *Dicts) ToYaml() ([]byte, error) {
	res, err := yaml.Marshal(d.items)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal the DataImportCronTemplate list; %v", err)
	}

	res = cleanup.CleanOutput(res)

	return res, nil
}
