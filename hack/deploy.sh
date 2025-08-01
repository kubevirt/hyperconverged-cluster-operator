#!/usr/bin/env bash
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
# Copyright 2017 Red Hat, Inc.
#

set -ex

source hack/common.sh
source hack/compare_scc.sh

HCO_IMAGE=${HCO_IMAGE:-quay.io/kubevirt/hyperconverged-cluster-operator:latest}
WEBHOOK_IMAGE=${WEBHOOK_IMAGE:-quay.io/kubevirt/hyperconverged-cluster-webhook:latest}
HCO_NAMESPACE="kubevirt-hyperconverged"
HCO_KIND="hyperconvergeds"
HCO_RESOURCE_NAME="kubevirt-hyperconverged"
HCO_CRD_NAME="hyperconvergeds.hco.kubevirt.io"
IS_OPENSHIFT=${IS_OPENSHIFT:-false}

if ${CMD}  api-resources |grep clusterversions |grep config.openshift.io; then
  IS_OPENSHIFT="true"
fi

CI=""
if [ "$1" == "CI" ]; then
	echo "deploying on CI"
	CI="true"
elif [ "$HOSTNAME" == "hco-e2e-aws" ] || [ "$HOSTNAME" == "e2e-aws-cnv" ]; then
	echo "deploying on AWS CI"
	CI="true"
fi

# Cleanup previously generated manifests
rm -rf _out/

# Copy release manifests as a base for generated ones, this should make it possible to upgrade
cp -r deploy _out/

# dump cluster SCCs to be sure we are not going to modify them with the deployment
dump_sccs_before

# if this is set we run on okd ci
if [ -n "${IMAGE_FORMAT}" ]; then
    component=hyperconverged-cluster-operator
    HCO_IMAGE=`eval echo ${IMAGE_FORMAT}`
fi

sed -i -r "s|: quay.io/kubevirt/hyperconverged-cluster-operator(@sha256)?:.*$|: ${HCO_IMAGE}|g" _out/operator.yaml
sed -i -r "s|: quay.io/kubevirt/hyperconverged-cluster-webhook(@sha256)?:.*$|: ${WEBHOOK_IMAGE}|g" _out/operator.yaml

# create namespaces
"${CMD}" create ns "${HCO_NAMESPACE}" | true

# Create additional namespaces needed for HCO components
namespaces=("openshift")
for namespace in ${namespaces[@]}; do
    if [[ $(${CMD} get ns ${namespace}) == "" ]]; then
        ${CMD} create ns ${namespace}
    fi
done

if [ "${IS_OPENSHIFT}" == "true" ]; then
    # Switch project to kubevirt-hyperconverged
    oc project "${HCO_NAMESPACE}"
else
    # switch namespace to kubevirt-hyperconverged
    ${CMD} config set-context $(${CMD} config current-context) --namespace="${HCO_NAMESPACE}"
fi

function status(){
    "${CMD}" get hco -n "${HCO_NAMESPACE}" -o yaml || true
    "${CMD}" get pods -n "${HCO_NAMESPACE}" || true
    "${CMD}" get hco "${HCO_RESOURCE_NAME}" -n "${HCO_NAMESPACE}" -o=jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.message}{"\n"}{end}' || true
    # Get logs of all the pods
    for PNAME in $( ${CMD} get pods -n ${HCO_NAMESPACE} --field-selector=status.phase!=Running -o custom-columns=:metadata.name )
    do
      echo -e "\n--- ${PNAME} ---"
      ${CMD} describe pod -n ${HCO_NAMESPACE} ${PNAME} || true
      ${CMD} logs -n ${HCO_NAMESPACE} ${PNAME} --all-containers=true || true
    done
    HCO_POD=$( ${CMD} get pods -l "name=hyperconverged-cluster-operator" -o name)
    ${CMD} logs "${HCO_POD}"
}

trap status EXIT

CONTAINER_ERRORED=""
function debug(){
    echo "Found pods with errors ${CONTAINER_ERRORED}"

    for err in ${CONTAINER_ERRORED}; do
        echo "------------- $err"
        "${CMD}" logs $("${CMD}" get pods -n "${HCO_NAMESPACE}" | grep $err | head -1 | awk '{ print $1 }')
    done
    exit 1
}

# Exclude Openshift specific resources
LABEL_SELECTOR_ARG=""
if [ "$IS_OPENSHIFT" != "true" ]; then
    LABEL_SELECTOR_ARG="-l name!=ssp-operator,name!=hyperconverged-cluster-cli-download"
fi

# Deploy cert-manager for webhooks
"${CMD}" apply -f _out/cert-manager.yaml
"${CMD}" -n cert-manager wait deployment/cert-manager-webhook --for=condition=Available --timeout="300s"

# Deploy local manifests
"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/cluster_role.yaml
"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/service_account.yaml
"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/cluster_role_binding.yaml
"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/crds/

sleep 20
if [[ "$(${CMD} get crd ${HCO_CRD_NAME} -o=jsonpath='{.status.conditions[?(@.type=="NonStructuralSchema")].status}')" == "True" ]];
then
    echo "HCO CRD reports NonStructuralSchema condition"
    "${CMD}" get crd ${HCO_CRD_NAME} -o go-template='{{ range .status.conditions }}{{ .type }}{{ "\t" }}{{ .status }}{{ "\t" }}{{ .message }}{{ "\n" }}{{ end }}'
fi

# note that generated certificates are necessary for webhook deployments
# manifest-templator does not add them into operator.yaml at the moment. 
# when a new webhook (deployed by OLM in production) is introduced, 
# it must be added into webhooks.yaml as well.

echo "Waiting for cert-manager webhook CA bundle..."
hack/retry.sh 60 1 \
    "${CMD} get validatingwebhookconfigurations cert-manager-webhook -o jsonpath='{.webhooks[0].clientConfig.caBundle}' | base64 -d | openssl x509 -noout 2>/dev/null" \
    "echo 'Warning: cert-manager webhook CA bundle still not valid after 60 attempts'"

if [ $? -eq 0 ]; then
    echo "cert-manager webhook CA bundle is valid!"
fi

echo "Creating resources for webhooks"
"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/webhooks.yaml

if [ "${CI}" != "true" ]; then
	"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/operator.yaml
else
	sed -E 's|^(\s*)- name: KVM_EMULATION$|\1- name: KVM_EMULATION\n\1  value: "true"|' < _out/operator.yaml > _out/operator-ci.yaml
	cat _out/operator-ci.yaml
	"${CMD}" apply $LABEL_SELECTOR_ARG -f _out/operator-ci.yaml
fi

# Wait for the HCO to be ready
sleep 20

"${CMD}" wait deployment/hyperconverged-cluster-operator --for=condition=Available --timeout="1080s" || CONTAINER_ERRORED+="hyperconverged-cluster-operator "
"${CMD}" wait deployment/hyperconverged-cluster-webhook --for=condition=Available --timeout="1080s" || CONTAINER_ERRORED+="hyperconverged-cluster-webhook "

# Gather a list of operators to wait for.
# Avoid checking the availability of virt-operator here because it will become available only when
# HCO will create its priorityClass and this will happen only when wi will have HCO cr.
# Check on ssp-operator only if it's enabled.
OPERATORS=(
    "cdi-operator"
    "cluster-network-addons-operator"
)

if [ "$IS_OPENSHIFT" = "true" ]; then
    OPERATORS+=("ssp-operator")
    OPERATORS+=("hyperconverged-cluster-cli-download")
fi

for op in "${OPERATORS[@]}"; do
    "${CMD}" wait deployment/"${op}" --for=condition=Available --timeout="540s" || CONTAINER_ERRORED+="${op} "
done

"${CMD}" apply -f _out/hco.cr.yaml
sleep 10
# Give 30 minutes to available condition become true
if ! ${CMD} wait -n ${HCO_NAMESPACE} ${HCO_KIND} ${HCO_RESOURCE_NAME} --for=condition=Available --timeout=30m;
then
    echo "Available condition never became true"
    "${CMD}" get pods -n "${HCO_NAMESPACE}"
    "${CMD}" get -n ${HCO_NAMESPACE} ${HCO_KIND} ${HCO_RESOURCE_NAME} -o yaml
    exit 1
fi
# Show all conditions and their status
"${CMD}" get -n ${HCO_NAMESPACE} ${HCO_KIND} ${HCO_RESOURCE_NAME} -o go-template='{{ range .status.conditions }}{{ .type }}{{ "\t" }}{{ .status }}{{ "\t" }}{{ .message }}{{ "\n" }}{{ end }}'

for dep in cdi-apiserver cdi-deployment cdi-uploadproxy virt-api virt-controller; do
    "${CMD}" wait deployment/"${dep}" --for=condition=Available --timeout="360s" || CONTAINER_ERRORED+="${dep} "
done

echo "Check how HCO detected the kind of cluster"
HCO_POD=$( ${CMD} get pods -n ${HCO_NAMESPACE} -l "name=hyperconverged-cluster-operator" --field-selector=status.phase=Running -o name | head -n1)
${CMD} logs -n ${HCO_NAMESPACE} "${HCO_POD}" | grep "Cluster type = "

# compare initial cluster SCCs to be sure HCO deployment didn't introduce any change
dump_sccs_after

if [ -z "$CONTAINER_ERRORED" ]; then
    echo "SUCCESS"
    exit 0
else
    CONTAINER_ERRORED+='hyperconverged-cluster-operator '
    debug
    "${CMD}" get pods -n "${HCO_NAMESPACE}"
fi
