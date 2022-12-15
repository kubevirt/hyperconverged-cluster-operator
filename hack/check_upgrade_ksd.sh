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
# Copyright 2022 Red Hat, Inc.
#

set -ex

if [[ -z ${PREVIOUS_KSD_ANNOTATION} && ${PREVIOUS_KSD_STATE} == '{}' ]] || [[ ${PREVIOUS_KSD_ANNOTATION} == 'true' ]]; then
  # if the annotation did not exist and KSD was running, or if it existed and equal to 'true' - KSD should be deployed in new version

  echo "check that KSD annotation in HCO CR in new version is set to true"
  [[ $(${CMD} get HyperConverged kubevirt-hyperconverged -n kubevirt-hyperconverged -o jsonpath='{.metadata.annotations.deployKSD}') == 'true' ]]

  echo "check that KSD exists in CNAO CR Spec"
  [[ $(${CMD} get networkaddonsconfigs cluster -o jsonpath='{.spec.kubeSecondaryDNS}') == '{}' ]]

  echo "check that KSD Deployment exists"
  [[ $(${CMD} get deployment secondary-dns -n kubevirt-hyperconverged --no-headers --ignore-not-found | wc -l) == '1' ]]

elif [[ -z ${PREVIOUS_KSD_ANNOTATION} && -z ${PREVIOUS_KSD_STATE} ]] || [[ -n ${PREVIOUS_KSD_ANNOTATION} && ${PREVIOUS_KSD_ANNOTATION} != 'true' ]]; then
  # if the annotation did not exist and KSD was not running,
  # or if the annotation existed and was not 'true' - KSD should not be deployed in new version

  echo "check that KSD annotation in HCO CR in new version is set to false"
  [[ $(${CMD} get HyperConverged kubevirt-hyperconverged -n kubevirt-hyperconverged -o jsonpath='{.metadata.annotations.deployKSD}') == 'false' ]]

  echo "check that KSD does not exist in CNAO CR Spec"
  [[ $(${CMD} get networkaddonsconfigs cluster -o jsonpath='{.spec.kubeSecondaryDNS}') == '' ]]

  echo "check that KSD Deployment does not exist"
  [[ $(${CMD} get ds deployment secondary-dns-n kubevirt-hyperconverged --no-headers --ignore-not-found | wc -l) == '0' ]]

else
  echo "KSD opt-in test did not run. PREVIOUS_KSD_ANNOTATION=${PREVIOUS_KSD_ANNOTATION}, PREVIOUS_KSD_STATE=${PREVIOUS_KSD_STATE}"
  exit 1
fi

echo "KSD Opt-in test completed successfully."

