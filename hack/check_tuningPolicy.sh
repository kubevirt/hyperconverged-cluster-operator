#!/bin/bash -x
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
# This script checks the tuningPolicy configuration

RATE_LIMITS=(
  ".spec.configuration.apiConfiguration.restClient.rateLimiter.tokenBucketRateLimiter"
  ".spec.configuration.controllerConfiguration.restClient.rateLimiter.tokenBucketRateLimiter"
  ".spec.configuration.handlerConfiguration.restClient.rateLimiter.tokenBucketRateLimiter"
  ".spec.configuration.webhookConfiguration.restClient.rateLimiter.tokenBucketRateLimiter"

)

# Delete it just in case is present from previous executions
./hack/retry.sh 1 3 "${KUBECTL_BINARY} delete cm static-policy -n \"${INSTALLED_NAMESPACE}\""

CONFIG_MAP='{\"apiVersion\": \"v1\", \"kind\": \"ConfigMap\", \"metadata\": { \"name\": \"static-policy\", \"labels\": { \"app\": \"kubevirt-hyperconverged\"} }, \"data\": { \"queryPerSeconds\": \"100\", \"burst\": \"200\" }}'

EXPECTED='{
  "burst": 200,
  "qps": 100
}'

echo "Check that the TuningPolicy ConfigMap is configuring the KV object as expected"

./hack/retry.sh 10 3 "(echo \"$CONFIG_MAP\" | ${KUBECTL_BINARY} create -f - -n \"${INSTALLED_NAMESPACE}\")"

./hack/retry.sh 10 3 "(${KUBECTL_BINARY} patch -n \"${INSTALLED_NAMESPACE}\" hco kubevirt-hyperconverged --type=json -p='[{"op": "add", "path": "/spec/tuningPolicy", "value": "static-policy"}]')"

for jpath in "${RATE_LIMITS[@]}"; do
  KUBECONFIG_OUT=$(${KUBECTL_BINARY} get kv -n "${INSTALLED_NAMESPACE}" kubevirt-kubevirt-hyperconverged -o json | jq "${jpath}")
  if [[ $KUBECONFIG_OUT != $EXPECTED ]]; then
     exit 1
  fi
  sleep 2
done