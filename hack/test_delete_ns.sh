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
# Copyright 2020 Red Hat, Inc.
#

set -ex

function test_delete_ns(){
    OLM=0
    if [ "${CMD}" == "oc" ]; then
      CSV=$(oc get csv -n kubevirt-hyperconverged -o name | wc -l)
      if [ "$CSV" -gt 0 ]; then
        OLM=1
      fi
    fi

    if [ "$OLM" -gt 0 ]; then
        # TODO: remove this once we are able to run the webhook also on k8s
        echo "HCO has been deployed by the OLM, so its webhook is supposed to work: let's test it"

        echo "kubevirt-hyperconverged namespace should be still there"
        ${CMD} get namespace kubevirt-hyperconverged -o yaml

        echo "Trying to delete kubevirt-hyperconverged namespace when the hyperconverged CR is still there"
        time timeout 60m ${CMD} delete namespace kubevirt-hyperconverged

    else
        # TODO: remove this once we are able to run the webhook also on k8s
        echo "Ignoring webhook on k8s where we don't have OLM based validating webhooks"

        echo "Delete the hyperconverged CR to remove the product"
        time timeout 30m ${CMD} delete hyperconverged -n kubevirt-hyperconverged kubevirt-hyperconverged
    
        echo "Finally delete kubevirt-hyperconverged namespace"
        time timeout 30m ${CMD} delete namespace kubevirt-hyperconverged
    fi

}

