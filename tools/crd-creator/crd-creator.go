package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"golang.org/x/tools/go/packages"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crdgen "sigs.k8s.io/controller-tools/pkg/crd"
	crdmarkers "sigs.k8s.io/controller-tools/pkg/crd/markers"
	"sigs.k8s.io/controller-tools/pkg/loader"
	"sigs.k8s.io/controller-tools/pkg/markers"

	hcov1beta1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1beta1"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
	"github.com/kubevirt/hyperconverged-cluster-operator/tools/util"
)

const (
	objectType = "object"
	importPath = "github.com/kubevirt/hyperconverged-cluster-operator/api/..."
)

var (
	fileName string
)

func init() {
	flag.StringVar(&fileName, "output-file", "", "CRD output file name")

	flag.Parse()
}

func main() {
	crd, err := getOperatorCRD()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to generate CRD", err)
		panic(err)
	}

	var output io.Writer
	if fileName == "" {
		output = os.Stdout
	} else {
		f, err := os.Create(fileName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create output file %s: %v\n", fileName, err)
			os.Exit(1)
		}
		defer f.Close()
		output = f
	}

	err = util.MarshallObject(crd, output)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to marshall CRD", err)
		panic(err)
	}
}

func getOperatorCRD() (*extv1.CustomResourceDefinition, error) {
	pkgs, err := loader.LoadRoots(importPath)
	if err != nil {
		return nil, err
	}
	reg := &markers.Registry{}
	err = crdmarkers.Register(reg)
	if err != nil {
		return nil, fmt.Errorf("failed to register CRD markers: %w", err)
	}

	parser := &crdgen.Parser{
		Collector:                  &markers.Collector{Registry: reg},
		Checker:                    &loader.TypeChecker{},
		GenerateEmbeddedObjectMeta: true,
	}

	crdgen.AddKnownTypes(parser)
	if len(pkgs) == 0 {
		panic("Failed identifying packages")
	}
	for _, p := range pkgs {
		parser.NeedPackage(p)
	}
	groupKind := schema.GroupKind{Kind: hcoutil.HyperConvergedKind, Group: hcoutil.APIVersionGroup}
	parser.NeedCRDFor(groupKind, nil)
	for _, p := range pkgs {
		err = packageErrors(p, packages.TypeError)
		if err != nil {
			panic(err)
		}
	}
	c := parser.CustomResourceDefinitions[groupKind]
	// enforce validation of CR name to prevent multiple CRs
	for _, v := range c.Spec.Versions {
		v.Schema.OpenAPIV3Schema.Properties["metadata"] = extv1.JSONSchemaProps{
			Type: objectType,
			Properties: map[string]extv1.JSONSchemaProps{
				"name": {
					Type:    "string",
					Pattern: hcov1beta1.HyperConvergedName,
				},
			},
		}
	}
	return &c, nil
}

func packageErrors(pkg *loader.Package, filterKinds ...packages.ErrorKind) error {
	toSkip := make(map[packages.ErrorKind]struct{})
	for _, errKind := range filterKinds {
		toSkip[errKind] = struct{}{}
	}
	var outErr error
	packages.Visit([]*packages.Package{pkg.Package}, nil, func(pkgRaw *packages.Package) {
		for _, err := range pkgRaw.Errors {
			if _, skip := toSkip[err.Kind]; skip {
				continue
			}
			outErr = err
		}
	})
	return outErr
}
