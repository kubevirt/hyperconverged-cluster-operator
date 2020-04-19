#!/bin/bash

set -x
# Setup Environment Variables
TARGET_NAMESPACE="${TARGET_NAMESPACE:-kubevirt-hyperconverged}"
HCO_VERSION="${HCO_VERSION:-1.1.0}"
HCO_CHANNEL="${HCO_CHANNEL:-1.1.0}"
MARKETPLACE_MODE="${MARKETPLACE_MODE:-true}"
HCO_REGISTRY_IMAGE="${HCO_REGISTRY_IMAGE:-quay.io/kubevirt/hco-container-registry:latest}"
PRIVATE_REPO="${PRIVATE_REPO:-false}"
QUAY_USERNAME="${QUAY_USERNAME:-}"
QUAY_PASSWORD="${QUAY_PASSWORD:-}"
QUAY_TOKEN=""
CREATE_NAMESPACE="${CREATE_NAMESPACE:-false}"
CONTENT_ONLY="${CONTENT_ONLY:-false}"
KVM_EMULATION="${KVM_EMULATION:-false}"
OC_TOOL="${OC_TOOL:-oc}"

#####################

SCRIPT_DIR="$(dirname "$0")"
source $SCRIPT_DIR/get_quay_token.sh

TMPDIR=$(mktemp -d)
cp -r $SCRIPT_DIR/* $TMPDIR

if [ "$PRIVATE_REPO" = 'true' ]; then
  get_quay_token
  oc create secret generic quay-registry-kubevirt-hyperconverged --from-literal=token="$QUAY_TOKEN" -n ${TARGET_NAMESPACE}

  cat <<EOF > $TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - private_repo
EOF
  oc apply -k $TMPDIR

  else # not private repo
    if [ "$MARKETPLACE_MODE" = 'true' ]; then
        cat <<EOF > $TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - marketplace
EOF
  oc apply -k $TMPDIR
  else
    cat <<EOF > $TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - image_registry
EOF
  oc apply -k $TMPDIR
  fi
fi

if [ "$CONTENT_ONLY" = 'true' ]; then
  echo INFO: Content is ready for deployment in OLM.
  exit 0
fi
source $SCRIPT_DIR/retry_loop.sh

# KVM_EMULATION setting is active only when a deployment is done.
if [ "$KVM_EMULATION" = 'true' ]; then
  cat <<EOF > $TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - kvm_emulation
resources:
  - namespace.yaml
EOF
  retry_loop $TMPDIR
else
  cat <<EOF > $TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - base
resources:
  - namespace.yaml
EOF
  retry_loop $TMPDIR
fi