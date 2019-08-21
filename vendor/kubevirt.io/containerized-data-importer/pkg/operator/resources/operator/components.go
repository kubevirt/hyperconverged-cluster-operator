/*
Copyright 2018 The CDI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

import (
	csvv1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	extv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

const (
	//ControllerImageDefault - default value
	ControllerImageDefault = "cdi-controller"
	//ImporterImageDefault - default value
	ImporterImageDefault = "cdi-importer"
	//ClonerImageDefault - default value
	ClonerImageDefault = "cdi-cloner"
	//APIServerImageDefault - default value
	APIServerImageDefault = "cdi-apiserver"
	//UploadProxyImageDefault - default value
	UploadProxyImageDefault = "cdi-uploadproxy"
	//UploadServerImageDefault - default value
	UploadServerImageDefault = "cdi-uploadserver"
	// OperatorImageDefault - default value
	OperatorImageDefault = "cdi-operator"
)

//NewClusterServiceVersionData - Data arguments used to create CDI's CSV manifest
type NewClusterServiceVersionData struct {
	CsvVersion         string
	ReplacesCsvVersion string
	Namespace          string
	ImagePullPolicy    string
	IconBase64         string
	Verbosity          string

	DockerPrefix string
	DockerTag    string

	CdiImageNames *CdiImages
}

//CdiImages - images to be provied to cdi operator
type CdiImages struct {
	ControllerImage   string
	ImporterImage     string
	ClonerImage       string
	APIServerImage    string
	UplodaProxyImage  string
	UplodaServerImage string
	OperatorImage     string
}

//FillDefaults - fill image names with defaults
func (ci *CdiImages) FillDefaults() *CdiImages {
	if ci.ControllerImage == "" {
		ci.ControllerImage = ControllerImageDefault
	}
	if ci.ImporterImage == "" {
		ci.ImporterImage = ImporterImageDefault
	}
	if ci.ClonerImage == "" {
		ci.ClonerImage = ClonerImageDefault
	}
	if ci.APIServerImage == "" {
		ci.APIServerImage = APIServerImageDefault
	}
	if ci.UplodaProxyImage == "" {
		ci.UplodaProxyImage = UploadProxyImageDefault
	}
	if ci.UplodaServerImage == "" {
		ci.UplodaServerImage = UploadServerImageDefault
	}
	if ci.OperatorImage == "" {
		ci.OperatorImage = OperatorImageDefault
	}

	return ci
}

//NewCdiOperatorDeployment - provides operator deployment spec
func NewCdiOperatorDeployment(namespace string, repository string, tag string, imagePullPolicy string, verbosity string, cdiImages *CdiImages) (*appsv1.Deployment, error) {
	deployment := createOperatorDeployment(
		repository,
		namespace,
		"true",
		cdiImages.OperatorImage,
		cdiImages.ControllerImage,
		cdiImages.ImporterImage,
		cdiImages.ClonerImage,
		cdiImages.APIServerImage,
		cdiImages.UplodaProxyImage,
		cdiImages.UplodaServerImage,
		tag,
		verbosity,
		imagePullPolicy)

	return deployment, nil
}

//NewCdiOperatorClusterRole - provides operator clusterRole
func NewCdiOperatorClusterRole() *rbacv1.ClusterRole {
	return createOperatorClusterRole(operatorClusterRoleName)
}

//NewCdiCrd - provides CDI CRD
func NewCdiCrd() *extv1beta1.CustomResourceDefinition {
	return createCDIListCRD()
}

//NewClusterServiceVersion - generates CSV for CDI
func NewClusterServiceVersion(data *NewClusterServiceVersionData) (*csvv1.ClusterServiceVersion, error) {
	return createClusterServiceVersion(data)
}
