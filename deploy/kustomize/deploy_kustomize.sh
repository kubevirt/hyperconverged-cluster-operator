#!/bin/bash

set -e
# Setup Environment Variables
export TARGET_NAMESPACE="${TARGET_NAMESPACE:-kubevirt-hyperconverged}"
export HCO_VERSION="${HCO_VERSION:-1.0.0}"
export HCO_CHANNEL="${HCO_CHANNEL:-1.0.0}"
export HCO_REGISTRY_IMAGE="${HCO_REGISTRY_IMAGE:-quay.io/kubevirt/hco-container-registry:latest}"
export PRIVATE_REPO="${PRIVATE_REPO:-false}"
export QUAY_USERNAME="${QUAY_USERNAME:-}"
export QUAY_PASSWORD="${QUAY_PASSWORD:-}"
export QUAY_TOKEN=""
export AUTH_TOKEN=""
export CREATE_NAMESPACE="${CREATE_NAMESPACE:-false}"
export CONTENT_ONLY="${CONTENT_ONLY:-false}"
export KVM_EMULATION="${KVM_EMULATION:-false}"

TEMPLATE_DIR=template

if [ ! "$1" = 'marketplace' ] && [ ! "$1" = 'image_registry' ]; then
  echo [ERROR] You must specify either "marketplace" or "image_registry" as an argument.
  exit 1
fi

# Get Quay token if private repo is chosen
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
    eval "$1='$token'";
  fi
}

if [ "$PRIVATE_REPO" = 'true' ]; then
  if [ -z "$QUAY_USERNAME" ] || [ -z "$QUAY_PASSWORD" ]; then
    echo [ERROR] Please set both "QUAY_USERNAME" and "QUAY_PASSWORD";
    exit 1;
  fi
  get_quay_token QUAY_TOKEN
  AUTH_TOKEN=$(cat <<-END
  authorizationToken:
    secretName: "quay-registry-${APP_REGISTRY}"
END
)
fi


cd "$(dirname "$0")"

OUTPUT_DIR=generated_manifests
rm -rf ${OUTPUT_DIR} && mkdir -p ${OUTPUT_DIR}

# Perform variables substitution
cd template
for file in **/*;
do
  mkdir -p ../$OUTPUT_DIR/`dirname ${file}`;
  envsubst < "$file" > ../$OUTPUT_DIR/"${file}";
done
cd ..

# Default apply directory
APPLY_DIR=$1

# If CONTENT_ONLY is enabled, remove the deployment base (Subscription, OperatorGroup and HCO-CR)
if [ "$CONTENT_ONLY" = 'true' ]; then
  sed -i "/customized/d" $OUTPUT_DIR/$1/kustomization.yaml

  # "Create Namespace" can be disabled only if we're not deploying HCO
  if [ "$CREATE_NAMESPACE" = 'false' ]; then
    sed -i "/namespace/d" $OUTPUT_DIR/customized/kustomization.yaml
  fi
fi

# If private repo is enabled, set the apply directory to "private_repo"
if [ "$PRIVATE_REPO" = 'true' ]; then
  APPLY_DIR=private_repo
fi

# If KVM_EMULATION is enabled, set the apply directory to "kvm_emulation" and set the proper base
if [ "$KVM_EMULATION" = 'true' ]; then
  KVM_KUST_FILE=$OUTPUT_DIR/kvm_emulation/kustomization.yaml
  if [ "$PRIVATE_REPO" = 'true' ]; then
    sed -i "s|\.\./.*|../private_repo|g" $KVM_KUST_FILE;
  else
    sed -i "s|\.\./.*|../$1|g" $KVM_KUST_FILE;
  fi
  APPLY_DIR=kvm_emulation
fi

APPLY_DIR=$OUTPUT_DIR/$APPLY_DIR

echo "[INFO] kustomize manifests to be applied are in `pwd`/$APPLY_DIR"

###################################################################################

if [ "$2" != "deploy" ]; then
  exit 0
fi

echo "Applying generated manifests to deploy HCO via OLM..."

# expect oc to be in PATH by default
export OC_TOOL="${OC_TOOL:-oc}"

# Deploy HCO and OLM Resources with retries
success=0
iterations=0
sleep_time=10
max_iterations=72 # results in 12 minutes timeout
until [[ $success -eq 1 ]] || [[ $iterations -eq $max_iterations ]]
do
  deployment_failed=0

    if [[ ! -d $APPLY_DIR ]]; then
      echo "[ERROR] Manifests do not exist. Aborting..."
      exit 1
    fi

    set +e
    if ! ${OC_TOOL} apply -k $APPLY_DIR
    then
      echo "[WARN] Deployment of HCO has failed."
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
  echo "[INFO] Deployment successful."
  if [ "$CONTENT_ONLY" = "true" ]; then
    echo "[INFO] Content created and ready for manual installation."
    exit 0
  else
    echo "[INFO] Deployment successful, waiting for HCO Operator to report Ready..."
    ${OC_TOOL} wait -n ${TARGET_NAMESPACE} hyperconverged kubevirt-hyperconverged --for condition=Available --timeout=15m
    ${OC_TOOL} wait "$(${OC_TOOL} get pods -n ${TARGET_NAMESPACE} -l name=hyperconverged-cluster-operator -o name)" -n "${TARGET_NAMESPACE}" --for condition=Ready --timeout=15m
  fi
else
  echo "[ERROR] Deployment failed."
  exit 1
fi
