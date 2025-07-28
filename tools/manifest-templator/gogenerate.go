package main

import _ "embed"

//go:generate go run ../crd-creator --output-file=generated-crd.yaml

//go:embed generated-crd.yaml
var crdBytes []byte
