#!/usr/bin/env bash
# Load build arguments directly from go.mod

source ./hack/architecture.sh
. ./hack/cri-bin.sh && export CRI_BIN=${CRI_BIN}

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
IMAGE_NAME=${IMAGE_NAME:-hco-builder}

# Extract K8S_VER from code-generator replace directive
K8S_VER=$(grep "k8s.io/code-generator => k8s.io/code-generator" go.mod | xargs | cut -d" " -f4)
    
# Extract KUBEOPENAPI_VER from kube-openapi replace directive
KUBEOPENAPI_VER=$(grep "k8s.io/kube-openapi => k8s.io/kube-openapi" go.mod | xargs | cut -d" " -f4)

echo "Loaded from go.mod: K8S_VER=${K8S_VER}, KUBEOPENAPI_VER=${KUBEOPENAPI_VER}"

if ${CRI_BIN} manifest exists "${IMAGE_NAME}"; then
  ${CRI_BIN} manifest rm "${IMAGE_NAME}"
fi
${CRI_BIN} manifest create "${IMAGE_NAME}"

for arch in ${ARCHITECTURES}; do
  ${CRI_BIN} build --platform="linux/${arch}" -t ${IMAGE_NAME}-${arch} -f hack/builder/Dockerfile --build-arg K8S_VER=${K8S_VER} --build-arg KUBEOPENAPI_VER=${KUBEOPENAPI_VER} ${script_dir}
  ./hack/retry.sh 3 10 "${CRI_BIN} push ${IMAGE_NAME}-${arch}"
  ${CRI_BIN} manifest add "${IMAGE_NAME}" "${IMAGE_NAME}-${arch}"
done

./hack/retry.sh 3 10 "${CRI_BIN} manifest push ${IMAGE_NAME}"
