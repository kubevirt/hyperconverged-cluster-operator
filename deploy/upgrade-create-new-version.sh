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
# Copyright 2019 Red Hat, Inc.
#

set -ex

LATEST_VERSION=$(ls -d /registry/kubevirt-hyperconverged/*/ | sort -r | head -1 | cut -d '/' -f 4); 
UPGRADE_VERSION=100.0.0

cp -rf /registry/kubevirt-hyperconverged/$LATEST_VERSION /registry/kubevirt-hyperconverged/$UPGRADE_VERSION

mv /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$LATEST_VERSION.clusterserviceversion.yaml /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml
sed -i "s|name: kubevirt-hyperconverged-operator.v$LATEST_VERSION|name: kubevirt-hyperconverged-operator.v$UPGRADE_VERSION|g" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml
export REPLACES_LINE=`grep "replaces" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml`
sed -i "s|$REPLACES_LINE|  replaces: kubevirt-hyperconverged-operator.v$LATEST_VERSION|g" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml
sed -i "s|  version: $LATEST_VERSION|  version: $UPGRADE_VERSION|g" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml

echo "KUBEVIRT_PROVIDER $KUBEVIRT_PROVIDER"
echo "OPENSHIFT_BUILD_NAESPACE $OPENSHIFT_BUILD_NAMESPACE"

if [ -n "${KUBEVIRT_PROVIDER}" ]; then
  sed -i "s|quay.io/kubevirt/hyperconverged-cluster-operator:latest|registry:5000/kubevirt/hyperconverged-cluster-operator:latest|g" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml
else
  sed -i "s|quay.io/kubevirt/hyperconverged-cluster-operator:latest|registry.svc.ci.openshift.org/$OPENSHIFT_BUILD_NAMESPACE/stable:hyperconverged-cluster-operator|g" /registry/kubevirt-hyperconverged/$UPGRADE_VERSION/kubevirt-hyperconverged-operator.v$UPGRADE_VERSION.clusterserviceversion.yaml
fi
sed -i "s|currentCSV: kubevirt-hyperconverged-operator.v$LATEST_VERSION|currentCSV: kubevirt-hyperconverged-operator.v$UPGRADE_VERSION|g" /registry/kubevirt-hyperconverged/kubevirt-hyperconverged.package.yaml
