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

source hack/common.sh

if [ "${CMD}" == "kubectl" ]; then
    # create namespaces
    kubectl create ns kubevirt
    kubectl create ns cdi

    # switch namespace to kubevirt
    kubectl config set-context $(kubectl config current-context) --namespace=kubevirt
else
    # Create projects
    oc new-project kubevirt
    oc new-project cdi
    
    # Switch project to kubevirt
    oc project kubevirt;
fi
# Deploy HCO manifests
"${CMD}" create -f deploy/
"${CMD}" create -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml
"${CMD}" create -f deploy/crds/hco_v1alpha1_hyperconverged_cr.yaml

# Create kubevirt-operator
"${CMD}" create -f "${KUBEVIRT_OPERATOR_URL}" || true

# Create cdi-operator
"${CMD}" create -f "${CDI_OPERATOR_URL}" || true
