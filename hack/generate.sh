#!/usr/bin/env bash
set -ex

K8S_VER=$(grep "k8s.io/api => k8s.io/api" go.mod | xargs | cut -d" " -f4)
KUBEOPENAPI_VER="$(grep "k8s.io/kube-openapi => k8s.io/kube-openapi" go.mod | xargs | cut -d" " -f4)"
PROJECT_ROOT="$(readlink -e "$(dirname "${BASH_SOURCE[0]}")"/../)"

PACKAGE=github.com/kubevirt/hyperconverged-cluster-operator
API_FOLDER=api
API_VERSIONS=(v1 v1beta1)

go install \
	k8s.io/code-generator/cmd/deepcopy-gen@${K8S_VER} \
	k8s.io/code-generator/cmd/defaulter-gen@${K8S_VER} \
	k8s.io/code-generator/cmd/conversion-gen@${K8S_VER}

go install \
	k8s.io/kube-openapi/cmd/openapi-gen@${KUBEOPENAPI_VER}

for API_VERSION in ${API_VERSIONS[@]}; do
  deepcopy-gen \
    --output-file zz_generated.deepcopy.go \
    --go-header-file "${PROJECT_ROOT}/hack/boilerplate.go.txt" \
    "${PACKAGE}/${API_FOLDER}/${API_VERSION}"

  defaulter-gen \
    --output-file zz_generated.defaults.go \
    --go-header-file "${PROJECT_ROOT}/hack/boilerplate.go.txt" \
    "${PACKAGE}/${API_FOLDER}/${API_VERSION}"

  openapi-gen \
    --output-file zz_generated.openapi.go \
    --go-header-file "${PROJECT_ROOT}/hack/boilerplate.go.txt" \
    --output-dir ${API_FOLDER}/${API_VERSION}/ \
    --output-pkg github.com/kubevirt/hyperconverged-cluster-operator/api/${API_VERSION} \
    "${PACKAGE}/${API_FOLDER}/${API_VERSION}"

  go fmt ${API_FOLDER}/${API_VERSION}/zz_generated.deepcopy.go
  go fmt ${API_FOLDER}/${API_VERSION}/zz_generated.defaults.go
  go fmt ${API_FOLDER}/${API_VERSION}/zz_generated.openapi.go
done

# generate auto conversion file
conversion-gen --output-file=zz_generated.conversion.go \
               --go-header-file=./hack/boilerplate.go.txt \
               ./api/v1beta1
