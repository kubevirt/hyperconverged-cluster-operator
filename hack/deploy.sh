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

HCO_IMAGE=${HCO_IMAGE:-quay.io/kubevirt/hyperconverged-cluster-operator:latest}
HCO_NAMESPACE="kubevirt-hyperconverged"
HCO_KIND="hyperconvergeds"
HCO_RESOURCE_NAME="hyperconverged-cluster"

CI=""
if [ "$1" == "CI" ]; then
	echo "deploying on CI"
	CI="true"
elif [ "$HOSTNAME" == "hco-e2e-aws" ]; then
	echo "deploying on AWS CI"
	CI="true"
fi

# Cleanup previously generated manifests
rm -rf _out/

# Copy release manifests as a base for generated ones, this should make it possible to upgrade
cp -r deploy _out/

# if this is set we run on okd ci
if [ -n "${IMAGE_FORMAT}" ]; then
    component=hyperconverged-cluster-operator
    HCO_IMAGE=`eval echo ${IMAGE_FORMAT}`
fi

sed -i "s#image: quay.io/kubevirt/hyperconverged-cluster-operator:latest#image: ${HCO_IMAGE}#g" _out/operator.yaml

# create namespaces
"${CMD}" create ns "${HCO_NAMESPACE}" | true

# Create additional namespaces needed for HCO components
namespaces=("openshift")
for namespace in ${namespaces[@]}; do
    if [[ $(${CMD} get ns ${namespace}) == "" ]]; then
        ${CMD} create ns ${namespace}
    fi
done

if [ "${CMD}" == "oc" ]; then
    # Switch project to kubevirt-hyperconverged
    oc project "${HCO_NAMESPACE}"
else
    # switch namespace to kubevirt-hyperconverged
    ${CMD} config set-context $(${CMD} config current-context) --namespace="${HCO_NAMESPACE}"
fi

function status(){
    "${CMD}" get hco -n "${HCO_NAMESPACE}" -o yaml
    "${CMD}" get pods -n "${HCO_NAMESPACE}"
    "${CMD}" get hco hyperconverged-cluster -n "${HCO_NAMESPACE}" -o=jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.message}{"\n"}{end}'
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

# machine CRD already exists on OKD clusters, so we do not want to deploy it again
if [[ $(${CMD} get crd machines.machine.openshift.io) != "" ]]; then
    rm -rf _out/crds/machine.crd.yaml
fi

# Deploy local manifests
"${CMD}" create -f _out/cluster_role.yaml
"${CMD}" create -f _out/service_account.yaml
"${CMD}" create -f _out/cluster_role_binding.yaml
"${CMD}" create -f _out/crds/
if [ "${CI}" != "true" ]; then
	"${CMD}" create -f _out/operator.yaml
else
	cat _out/operator.yaml | sed 's|# enable only on CI|"true"|' > _out/operator-ci.yaml
	cat _out/operator-ci.yaml
	"${CMD}" create -f _out/operator-ci.yaml
fi

# Wait for the HCO to be ready
sleep 20

"${CMD}" wait deployment/hyperconverged-cluster-operator --for=condition=Available --timeout="720s" || CONTAINER_ERRORED+="${op}"

for op in cdi-operator cluster-network-addons-operator kubevirt-ssp-operator node-maintenance-operator virt-operator; do
    "${CMD}" wait deployment/"${op}" --for=condition=Available --timeout="360s" || CONTAINER_ERRORED+="${op} "
done

"${CMD}" create -f _out/hco.cr.yaml
sleep 10
# Give 8 minutes to available condition become true
if ! timeout 8m bash -c -- "until "${CMD}" get -n ${HCO_NAMESPACE} ${HCO_KIND} ${HCO_RESOURCE_NAME} -o go-template='{{ range .status.conditions }}{{ if eq .type \"Available\" }}{{ .status }}{{ end }}{{ end }}' | grep True; do sleep 1; done";
then
    echo "Available condition never became true"
    "${CMD}" get pods -n "${HCO_NAMESPACE}"
    exit 1
fi
# Show all conditions and their status
"${CMD}" get -n ${HCO_NAMESPACE} ${HCO_KIND} ${HCO_RESOURCE_NAME} -o go-template='{{ range .status.conditions }}{{ .type }}{{ "\t" }}{{ .status }}{{ "\t" }}{{ .message }}{{ "\n" }}{{ end }}'

for dep in cdi-apiserver cdi-deployment cdi-uploadproxy virt-api virt-controller; do
    "${CMD}" wait deployment/"${dep}" --for=condition=Available --timeout="360s" || CONTAINER_ERRORED+="${dep} "
done

if [ -z "$CONTAINER_ERRORED" ]; then
    echo "SUCCESS"
    exit 0
else
    CONTAINER_ERRORED+='hyperconverged-cluster-operator'
    debug
    "${CMD}" get pods -n "${HCO_NAMESPACE}"
fi
