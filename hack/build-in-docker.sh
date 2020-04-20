#!/usr/bin/env bash

set -e

source hack/common.sh
HCO_DIR="$(readlink -f $(dirname $0)/../)"
BUILD_DIR=${HCO_DIR}/tests/build
BUILD_TAG="hco-test-build"
REGISTRY="quay.io/kubevirt-hyperconverged"
TAG=latest
TEST_BUILD_TAG="${REGISTRY}/${BUILD_TAG}:${TAG}"


# Build the encapsulated compile and test container
(cd ${BUILD_DIR} && docker build --tag ${TEST_BUILD_TAG} .)

docker push ${TEST_BUILD_TAG}