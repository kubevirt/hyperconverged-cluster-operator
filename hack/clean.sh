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

# Delete kubevirt-operator
"${CMD}" delete crd kubevirts.kubevirt.io
"${CMD}" delete -n kubevirt apiservice v1alpha3.subresources.kubevirt.io
"${CMD}" delete -f "${KUBEVIRT_OPERATOR_URL}"

# Remove HCO manifests
"${CMD}" delete -f deploy/
"${CMD}" delete -f deploy/crds/hco_v1alpha1_hyperconverged_crd.yaml

# Delete cdi-operator
"${CMD}" delete -n cdi apiservice v1alpha1.cdi.kubevirt.io
"${CMD}" delete -f "${CDI_OPERATOR_URL}"
