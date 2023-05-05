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
# - Following variables to build container images and push them to a container registry.
#   - QUAY_USERNAME: username to be used to login to the quay registry
#   - QUAY_PASSWORD: password to authenticaye QUAY_USERNAME with quay registry. Helpful documentation - https://docs.quay.io/glossary/robot-accounts.html
#   - IMAGE_TAG: The tag to be used for index image
#   - REGISTRY_NAMESPACE: Your namespace on quay.io
#   - IMAGE_REGISTRY: quay.io or the image registry you're using
#   - CONTAINER_TAG: The tag to be used for hyperconverged-cluster-operator image
# - Access to a k8s/OCP cluster that has OLM enabled on it.

function wait_for_hco_deployment() {
    # sleep till deployment for hco-operator is started
    until kubectl get deployments hco-operator -n $1
    do
      sleep 5
    done
}

# tag for the bundle image
CONTAINER_TAG="${CONTAINER_TAG:-$(cat /dev/urandom | tr -dc 'a-zA-Z0-9' | fold -w 8 | head -n 1)}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-quay.io}"
OPERATOR_GROUP_NAME=kubevirt-hyperconverged-group
VERSION=1.10.0
IMAGE_PULL_SECRET=my-pull-secret
AUTHFILE=`mktemp --dry-run`

if [ -z "${REGISTRY_NAMESPACE}" ]; then
  echo "Please set REGISTRY_NAMESPACE"
  echo "   REGISTRY_NAMESPACE=kubevirt make cluster-sync-olm"
  exit 1
fi

if [ -z "${CSV_VERSION}" ]; then
  CSV_VERSION=latest
fi
# Image to be used in CSV manifests
HCO_OPERATOR_IMAGE=$IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-operator:$CONTAINER_TAG
HCO_OPERATOR_IMAGE=$HCO_OPERATOR_IMAGE CSV_VERSION=$CSV_VERSION make build-manifests
make bundleRegistry
./hack/build-index-image.sh $CONTAINER_TAG


# namespace to create catalog source in
CATSRC_NAMESPACE=""
CATSRC_NAME="test-hco-catalogsource"
# namespace to create subscription in
SUB_NAMESPACE="kubevirt-hyperconverged"
SUB_NAME="hco-operatorhub"

cluster=$(kubectl get ns openshift-operators 2>/dev/null)
if [ -z "$cluster" ]
then
  # it's a kubernetes cluster
  CATSRC_NAMESPACE="olm"
else
  # it's an OCP cluster
  CATSRC_NAMESPACE="openshift-marketplace"
fi

# OCP cluster already has a pull secret for quay.io in its global pull secret.
# So login to quay.io/<user-namespace> and copy the auth info to OCP as a pull secret.
podman login $IMAGE_REGISTRY/$REGISTRY_NAMESPACE -u $QUAY_USERNAME -p $QUAY_PASSWORD --authfile $AUTHFILE
# create an image pull secret so that image pull from quay.io works OOTB
kubectl create secret generic $IMAGE_PULL_SECRET -n $CATSRC_NAMESPACE --from-file=.dockerconfigjson=$AUTHFILE --type=kubernetes.io/dockerconfigjson

catsrc=$(kubectl get -n $CATSRC_NAMESPACE catsrc $CATSRC_NAME 2>/dev/null)
if [ -z "$catsrc" ]
then
  cat <<EOF | kubectl apply -f -
        apiVersion: operators.coreos.com/v1alpha1
        kind: CatalogSource
        metadata:
          name: $CATSRC_NAME
          namespace: $CATSRC_NAMESPACE
        spec:
          sourceType: grpc
          image: $IMAGE_REGISTRY/$REGISTRY_NAMESPACE/hyperconverged-cluster-index:$VERSION
          displayName: KubeVirt HyperConverged
          publisher: Red Hat
          secrets:
          - $IMAGE_PULL_SECRET
EOF
fi

kubectl create namespace $SUB_NAMESPACE 2>/dev/null
kubectl get secret $IMAGE_PULL_SECRET -n $CATSRC_NAMESPACE -o yaml | grep -v '^\s*namespace:\s' | kubectl apply --namespace $SUB_NAMESPACE -f -

cat <<EOF | oc apply -f -
      apiVersion: operators.coreos.com/v1
      kind: OperatorGroup
      metadata:
          name: $OPERATOR_GROUP_NAME
          namespace: $SUB_NAMESPACE
EOF

# create subscription since it doesn't exist
cat <<EOF | kubectl apply -f -
      apiVersion: operators.coreos.com/v1alpha1
      kind: Subscription
      metadata:
          name: $SUB_NAME
          namespace: $SUB_NAMESPACE
      spec:
          source: $CATSRC_NAME
          sourceNamespace: openshift-marketplace
          name: community-kubevirt-hyperconverged
          channel: $VERSION
EOF

wait_for_hco_deployment $SUB_NAMESPACE

kubectl patch -n kubevirt-hyperconverged deployment hco-operator --patch '{"spec": {"template": {"spec": {"imagePullSecrets": [{"name": "'${IMAGE_PULL_SECRET}'"}]}}}}'