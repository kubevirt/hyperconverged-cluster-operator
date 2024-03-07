#!/bin/bash

set -ex -o pipefail -o errtrace -o functrace

source vars

# build the container images and push them to registry
make container-build container-push

# Image to be used in CSV manifests
HCO_OPERATOR_IMAGE=$HCO_OPERATOR_IMAGE CSV_VERSION=$CSV_VERSION make build-manifests
sed -i "s|+WEBHOOK_IMAGE_TO_REPLACE+|${HCO_WEBHOOK_IMAGE}|g" deploy/index-image/community-kubevirt-hyperconverged/1.12.0/manifests/kubevirt-hyperconverged-operator.v1.12.0.clusterserviceversion.yaml
sed -i "s|+ARTIFACTS_SERVER_IMAGE_TO_REPLACE+|${ARTIFACTS_SERVER_IMAGE}|g" deploy/index-image/community-kubevirt-hyperconverged/1.12.0/manifests/kubevirt-hyperconverged-operator.v1.12.0.clusterserviceversion.yaml


operator-sdk generate bundle --input-dir deploy/index-image/ --output-dir _out/bundle
operator-sdk bundle validate _out/bundle  # optional
podman build -f deploy/index-image/bundle.Dockerfile -t $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:$IMAGE_TAG
podman push $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:$IMAGE_TAG
#operator-sdk run bundle $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:$IMAGE_TAG

