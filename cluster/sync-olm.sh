#!/bin/bash -x

# sync-olm syncs local code changes to the k8s/OCP cluster with OLM enabled on it.
# It aims to be the single command that a dev needs to run to sync changes
# instead of manually running a combination of below commands:
# - make build-operator
# - make build-webhook
# - make build-manifests
# - make container-build
# - make container-push
# - Replacing image names in couple of files
# - make bundleRegistry
# - ./hack/build-index-image.sh
#
# Prerequisites:
# - Bunch of variables to build container images and push them to a container registry.
# - Access to a k8s/OCP cluster that has OLM enabled on it.

# tag for the bundle image
CONTAINER_TAG="${CONTAINER_TAG:-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 8 | head -n 1)}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-docker.io}"
# image to be replaced in CSV yaml files
IMAGE_TO_REPLACE=$IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-operator:$CONTAINER_TAG
OPERATOR_GROUP_NAME=kubevirt-hyperconverged-group

function create_catsrc() {
  cat <<EOF | kubectl create -f -
      apiVersion: operators.coreos.com/v1alpha1
      kind: CatalogSource
      metadata:
        name: $CATSRC_NAME
        namespace: $CATSRC_NAMESPACE
      spec:
        sourceType: grpc
        image: $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:1.10.0
        displayName: KubeVirt HyperConverged
        publisher: Red Hat
EOF
}

function create_og() {
    cat <<EOF | oc apply -f -
    apiVersion: operators.coreos.com/v1
    kind: OperatorGroup
    metadata:
        name: $OPERATOR_GROUP_NAME
        namespace: $SUB_NAMESPACE
EOF
}

function create_subscription() {
  cat <<EOF | kubectl create -f -
      apiVersion: operators.coreos.com/v1alpha1
      kind: Subscription
      metadata:
          name: $SUB_NAME
          namespace: $SUB_NAMESPACE
      spec:
          source: $CATSRC_NAME
          sourceNamespace: openshift-marketplace
          name: community-kubevirt-hyperconverged
          channel: "1.10.0"
EOF
}

if [ -z "${REGISTRY_NAMESPACE}" ]; then
  echo "Please set REGISTRY_NAMESPACE"
  echo "   REGISTRY_NAMESPACE=rthallisey make cluster-sync-olm"
  echo "   make cluster-sync-olm REGISTRY_NAMESPACE=rthallisey"
  exit 1
fi

make build-operator
make build-webhook
make container-build
make container-push

if [ -z "${CSV_VERSION}"]; then
  CSV_VERSION=latest
fi
# Image to be used in CSV manifests
HCO_OPERATOR_IMAGE=$IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-operator
HCO_OPERATOR_IMAGE=$HCO_OPERATOR_IMAGE CSV_VERSION=$CSV_VERSION make build-manifests
make bundleRegistry
./hack/build-index-image.sh IMAGE_TAG

# namespace to create catalog source in
CATSRC_NAMESPACE=""
CATSRC_NAME="test-hco-catalogsource"
# namespace to create subscription in
SUB_NAMESPACE="kubevirt-hyperconverged"
SUB_NAME="hco-operatorhub"
CSV_NAME="kubevirt-hyperconverged-operator.v1.10.0"

cluster=$(kubectl get ns openshift-operators 2>/dev/null)
if [ -z "$cluster" ]
then
  # it's a kubernetes cluster
  CATSRC_NAMESPACE="olm"
else
  # it's an OCP cluster
  CATSRC_NAMESPACE="openshift-marketplace"
fi

catsrc=$(kubectl get -n $CATSRC_NAMESPACE catsrc $CATSRC_NAME 2>/dev/null)
if [ -z "$catsrc" ]
then
  create_catsrc
else
  kubectl delete -n $CATSRC_NAMESPACE catsrc $CATSRC_NAME
  create_catsrc
fi

kubectl create namespace kubevirt-hyperconverged 2>/dev/null

og=$(kubectl get -n $SUB_NAMESPACE og $OPERATOR_GROUP_NAME)
if [ -z "$og" ]
then
  create_og
fi

sub=$(kubectl get -n $SUB_NAMESPACE subscription $SUB_NAME 2>/dev/null)
if [ -z "$sub" ]
then
  # create subscription since it doesn't exist
  create_subscription
else
  # delete the subscription and the CSV, and create sub again
  kubectl delete -n $SUB_NAMESPACE subscription $SUB_NAME
  kubectl delete -n $SUB_NAMESPACE csv $CSV_NAME
  create_subscription
fi