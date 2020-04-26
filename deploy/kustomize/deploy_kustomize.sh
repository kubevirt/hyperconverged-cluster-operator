#!/bin/bash

#set -x
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
CONTENT_ONLY="${CONTENT_ONLY:-false}"
KVM_EMULATION="${KVM_EMULATION:-false}"
OC_TOOL="${OC_TOOL:-oc}"

#####################

main() {
  SCRIPT_DIR="$(dirname "$0")"
  PATCHES_DIR="patches"

  TMPDIR=$(mktemp -d)
  cp -r $SCRIPT_DIR/* $TMPDIR
  echo temp dir is: $TMPDIR

  if [ "$PRIVATE_REPO" = 'true' ]; then
    get_quay_token
    oc create secret generic quay-registry-kubevirt-hyperconverged --from-literal=token="$QUAY_TOKEN" -n ${TARGET_NAMESPACE}

    cat <<EOF >$TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - private_repo
EOF
    oc apply -k $TMPDIR

  else # not private repo
    if [ "$MARKETPLACE_MODE" = 'true' ]; then
      cat <<EOF >$TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - marketplace
EOF
      oc apply -k $TMPDIR
    else
      cat <<EOF >$TMPDIR/kustomization.yaml
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

  # If the user has defined overlay patches - insert them into kustomization.yaml
  if [ -d $TMPDIR/$PATCHES_DIR ]; then
    PATCHES="patchesStrategicMerge:"
    for file in $TMPDIR/$PATCHES_DIR/*
    do
      PATCHES+=$'\n  - '$PATCHES_DIR/`basename $file`
    done
  fi
  # KVM_EMULATION setting is active only when a deployment is done.
  if [ "$KVM_EMULATION" = 'true' ]; then
    cat <<EOF >$TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - kvm_emulation
resources:
  - namespace.yaml
$PATCHES
EOF
    exit
    retry_loop $TMPDIR
  else
    cat <<EOF >$TMPDIR/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

bases:
  - base
resources:
  - namespace.yaml
$PATCHES
EOF
    retry_loop $TMPDIR
  fi
}

get_quay_token() {
    token=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
  {
      "user": {
          "username": "'"${QUAY_USERNAME}"'",
          "password": "'"${QUAY_PASSWORD}"'"
      }
  }' | jq -r '.token')

  if [ "$token" == "null" ]; then
    echo [ERROR] Got invalid Token from Quay. Please check your credentials in QUAY_USERNAME and QUAY_PASSWORD.
    exit 1
  else
    QUAY_TOKEN=\"$token\";
  fi
}

# Deploy HCO and OLM Resources with retries
retry_loop() {
  success=0
  iterations=0
  sleep_time=10
  max_iterations=72 # results in 12 minutes timeout
  until [[ $success -eq 1 ]] || [[ $iterations -eq $max_iterations ]]
  do
    deployment_failed=0

      if [[ ! -d $1 ]]; then
        echo $1
        echo "[ERROR] Manifests do not exist. Aborting..."
        exit 1
      fi

      set +e
      if ! ${OC_TOOL} apply -k $1
      then
        deployment_failed=1
      fi
      set -e

    if [[ deployment_failed -eq 1 ]]; then
      iterations=$((iterations + 1))
      iterations_left=$((max_iterations - iterations))
      echo "[WARN] At least one deployment failed, retrying in $sleep_time sec, $iterations_left retries left"
      sleep $sleep_time
      continue
    fi
    success=1
  done

  if [[ $success -eq 1 ]]; then
    echo "[INFO] Deployment successful, waiting for HCO Operator to report Ready..."
    ${OC_TOOL} wait -n ${TARGET_NAMESPACE} hyperconverged kubevirt-hyperconverged --for condition=Available --timeout=15m
    ${OC_TOOL} wait "$(${OC_TOOL} get pods -n ${TARGET_NAMESPACE} -l name=hyperconverged-cluster-operator -o name)" -n "${TARGET_NAMESPACE}" --for condition=Ready --timeout=15m
  else
    echo "[ERROR] Deployment failed."
    exit 1
  fi
}

main