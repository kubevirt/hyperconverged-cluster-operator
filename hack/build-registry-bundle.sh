#!/bin/bash

IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"
REGISTRY_NAMESPACE="${REGISTRY_NAMESPACE:-}"
CONTAINER_TAG="${CONTAINER_TAG:-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 8 | head -n 1)}"
CONTAINER_BUILD_CMD="${CONTAINER_BUILD_CMD:-docker}"
CONTAINER_BUILD_EXTRA_FLAGS="${CONTAINER_BUILD_EXTRA_FLAGS:-}"

if [ -z "${REGISTRY_NAMESPACE}" ]; then
    echo "Please set REGISTRY_NAMESPACE"
    echo "   REGISTRY_NAMESPACE=rthallisey ./hack/build-registry-bundle.sh"
    echo "   make bundle-registry REGISTRY_NAMESPACE=rthallisey"
    exit 1
fi

TMP_ROOT="$(dirname "${BASH_SOURCE[@]}")/.."
REPO_ROOT=$(readlink -e "${TMP_ROOT}" 2> /dev/null || perl -MCwd -e 'print Cwd::abs_path shift' "${TMP_ROOT}")

pushd ${REPO_ROOT}/deploy
$CONTAINER_BUILD_CMD build --no-cache $CONTAINER_BUILD_EXTRA_FLAGS -t ${IMAGE_REGISTRY}/${REGISTRY_NAMESPACE}/hco-registry:${CONTAINER_TAG} -f Dockerfile .
$CONTAINER_BUILD_CMD push $CONTAINER_BUILD_EXTRA_FLAGS ${IMAGE_REGISTRY}/${REGISTRY_NAMESPACE}/hco-registry:${CONTAINER_TAG}
popd
