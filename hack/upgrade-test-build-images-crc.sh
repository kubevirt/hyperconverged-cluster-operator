#!/bin/bash -ex
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2019 Red Hat, Inc.
#
# Builds operator and registry images for CRC

registry=default-route-openshift-image-registry.apps-crc.testing

echo "INFO: registry: $registry"

export REGISTRY_NAMESPACE=kubevirt
export IMAGE_REGISTRY=$registry
export CONTAINER_TAG=latest
# uses the .ci version because it has KVM_EMULATION set to true
export REGISTRY_DOCKERFILE="Dockerfile.registry.ci" 
export REGISTRY_EXTRA_PUSH_ARGS=--tls-verify=false
make container-build-operator container-push-operator bundleRegistry

# check images are accessible
oc get imagestreams -n kubevirt hyperconverged-cluster-operator
oc get imagestreams -n kubevirt hco-registry


# Build upgrade registry image
export REGISTRY_DOCKERFILE="Dockerfile.registry.upgrade"
export REGISTRY_IMAGE_NAME="hco-registry-upgrade"
export REGISTRY_EXTRA_BUILD_ARGS="--build-arg FOR_CRC=true"
make bundleRegistry

pwd
make container-clusterserviceversion
ls -al ./test-out

# check images are accessible
oc get imagestreams -n kubevirt hco-registry-upgrade

