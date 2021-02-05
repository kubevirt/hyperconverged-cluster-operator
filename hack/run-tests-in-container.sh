#!/usr/bin/env bash

set -euo pipefail

INSTALLED_NAMESPACE=${INSTALLED_NAMESPACE:-"kubevirt-hyperconverged"}

source hack/common.sh
source cluster/kubevirtci.sh

if [ "${JOB_TYPE}" == "stdci" ]; then
    KUBECONFIG=$(kubevirtci::kubeconfig)
    source ./hack/upgrade-stdci-config
fi

if [[ ${JOB_TYPE} = "prow" ]]; then
    export KUBECTL_BINARY="oc"
else
    export KUBECTL_BINARY="cluster/kubectl.sh"
fi

operator_image="$($KUBECTL_BINARY -n "${INSTALLED_NAMESPACE}" get deploy hyperconverged-cluster-operator -o jsonpath='{.spec .template .spec .containers[?(@.name=="hyperconverged-cluster-operator")] .image}')"
functest_image="${operator_image//hyperconverged-cluster-operator/hyperconverged-cluster-functest}"

echo "Running tests with $functest_image"

$KUBECTL_BINARY -n "${INSTALLED_NAMESPACE}" create serviceaccount functest \
  --dry-run -o yaml  |$KUBECTL_BINARY apply -f -

$KUBECTL_BINARY create clusterrolebinding functest-cluster-admin \
    --clusterrole=cluster-admin \
    --serviceaccount="${INSTALLED_NAMESPACE}":functest \
    --dry-run -o yaml  |$KUBECTL_BINARY apply -f -

# don't forget to remove this!
functest_image="quay.io/erkanerol/hyperconverged-cluster-functest:v12"

$KUBECTL_BINARY -n "${INSTALLED_NAMESPACE}" delete pod functest --ignore-not-found --wait=true

$KUBECTL_BINARY -n "${INSTALLED_NAMESPACE}" run functest \
  --image="$functest_image" --serviceaccount=functest \
  --env="INSTALLED_NAMESPACE=${INSTALLED_NAMESPACE}" \
  --restart=Never