#!/bin/bash -ex

HCO_NAMESPACE="kubevirt-hyperconverged"
HCO_KIND="hyperconvergeds"
HCO_RESOURCE_NAME="kubevirt-hyperconverged"
HCO_DEPLOYMENT_NAME=hco-operator

echo "KUBEVIRT_PROVIDER: $KUBEVIRT_PROVIDER"

if [ -n "$KUBEVIRT_PROVIDER" ]; then
  echo "Running on STDCI ${KUBEVIRT_PROVIDER}"
  source ./hack/upgrade-stdci-config
else
  echo "Running on OpenShift CI"
  source ./hack/upgrade-openshiftci-config
fi

function cleanup() {
    rv=$?
    if [ "x$rv" != "x0" ]; then
        echo "Error during HCO CR deployment: exit status: $rv"
        make dump-state
        echo "*** HCO CR deployment failed ***"
    fi
    exit $rv
}

trap "cleanup" INT TERM EXIT

WORKERS=$(${CMD} get nodes -l "node-role.kubernetes.io/master!=" -o name)
WORKERS_ARR=(${WORKERS})

mkdir -p _out
cp deploy/hco.cr.yaml _out/

if [[ ${#WORKERS_ARR[@]} -ge 2 ]]; then
  # Set all the workers as "infra", except for the last one that is set to "workloads"
  for (( i=0; i<${#WORKERS_ARR[@]}-1; i++)); do
    ${CMD} label ${WORKERS_ARR[$i]} "node.kubernetes.io/hco-test-node-type=infra"
  done
  ${CMD} label ${WORKERS_ARR[$((${#WORKERS_ARR[@]}-1))]} "node.kubernetes.io/hco-test-node-type=workloads"

  hack/np-config-hook.sh
fi

${CMD} get nodes -o wide --show-labels

if [[ -n "${KVM_EMULATION}" ]]; then
  SUBSCRIPTION_NAME=$(oc get subscription -n "${HCO_NAMESPACE}" -o name)
  # cut the type prefix, e.g. subscription.operators.coreos.com/kubevirt-hyperconverged => kubevirt-hyperconverged
  SUBSCRIPTION_NAME=${SUBSCRIPTION_NAME/*\//}

  TMP_DIR=$(mktemp -d)
  cat > "${TMP_DIR}/subscription-patch.yaml" << EOF
spec:
  config:
    selector:
      matchLabels:
        name: hyperconverged-cluster-operator
    env:
    - name: 'KVM_EMULATION'
      value: "${KVM_EMULATION}"
EOF

  ${CMD} patch -n "${HCO_NAMESPACE}" Subscription "${SUBSCRIPTION_NAME}" --patch="$(cat "${TMP_DIR}/subscription-patch.yaml")" --type=merge

  # give it some time to take place
  sleep 60
  # wait for the HCO to run with the new configurations
  ${CMD} wait deployment ${HCO_DEPLOYMENT_NAME} --for condition=Available -n ${HCO_NAMESPACE} --timeout="1200s"
fi

PATCH="[{\"op\":\"add\",\"path\":\"/spec/configuration/developerConfiguration/logVerbosity\",\"value\":{\"virtAPI\":4,\"virtController\":4,\"virtHandler\":4,\"virtLauncher\":4,\"virtOperator\":4}}]"
sed "/^metadata:$/a\  annotations:\n    kubevirt.kubevirt.io/jsonpatch: '${PATCH}'" _out/hco.cr.yaml

${CMD} apply -n kubevirt-hyperconverged -f _out/hco.cr.yaml

${CMD} wait -n "${HCO_NAMESPACE}" "${HCO_KIND}" "${HCO_RESOURCE_NAME}" --for condition=Available --timeout="30m"
