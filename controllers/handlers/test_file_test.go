package handlers

import _ "embed"

//go:embed testFiles/dashboards/kubevirt-top-consumers.yaml
var kubevirtTopConsumersFileContent []byte

//go:embed testFiles/imageStreams/imageStream.yaml
var imageStreamFileContent []byte

//go:embed testFiles/quickstarts/quickstart.yaml
var quickstartFileContent []byte
