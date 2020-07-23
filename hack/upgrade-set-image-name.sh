#!/usr/bin/env bash
set -ex

UPGRADE_CSV_DIR="/registry/kubevirt-hyperconverged/${UPGRADE_VERSION}"
CSV_FILE="${UPGRADE_CSV_DIR}/kubevirt-hyperconverged-operator.v${UPGRADE_VERSION}.clusterserviceversion.yaml"

if [ -n "$KUBEVIRT_PROVIDER" ]; then
  sed -i "s|quay.io/kubevirt/hyperconverged-cluster-operator:.*$|registry:5000/kubevirt/hyperconverged-cluster-operator:latest|g" "${CSV_FILE}";
else
  sed -i "s|quay.io/kubevirt/hyperconverged-cluster-operator:.*$|registry.svc.ci.openshift.org/${OPENSHIFT_BUILD_NAMESPACE}/stable:hyperconverged-cluster-operator|g" "${CSV_FILE}";
fi
