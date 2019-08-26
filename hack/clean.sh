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

# Remove HCO
"${CMD}" delete -f deploy/hco.cr.yaml --wait=false --ignore-not-found || true
"${CMD}" wait --for=delete hyperconverged.hco.kubevirt.io/hyperconverged-cluster || true
"${CMD}" delete -f deploy/crds/hco.crd.yaml --wait=false --ignore-not-found || true

# Remove other settings
"${CMD}" delete -f deploy/cluster_role_binding.yaml --wait=false --ignore-not-found || true
"${CMD}" delete -f deploy/cluster_role.yaml --wait=false --ignore-not-found || true
"${CMD}" delete -f deploy/service_account.yaml --wait=false --ignore-not-found || true

# Delete namespace at the end
"${CMD}" delete ns kubevirt-hyperconverged --ignore-not-found || true