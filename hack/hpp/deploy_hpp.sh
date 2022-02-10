#!/bin/bash

#
# Configures HPP on an OCP cluster:
# - on regular clusters, HPP is deployed the legacy way
# - on SNO clusters, HPP is deployed using the StoragePool feature
#

set -ex

readonly SCRIPT_DIR=$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")

CLUSTER_TOPOLOGY=$(
  oc get infrastructure cluster \
    --output=jsonpath='{$.status.controlPlaneTopology}'
)

if [[ "${CLUSTER_TOPOLOGY}" != 'SingleReplica' ]]; then
  "${SCRIPT_DIR}"/configure_hpp_legacy.sh
else
  "${SCRIPT_DIR}"/configure_hpp_pool.sh
fi
