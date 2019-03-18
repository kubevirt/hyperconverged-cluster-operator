#!/bin/bash

set -e

TMP_ROOT="$(dirname "${BASH_SOURCE[@]}")/.."
REPO_ROOT=$(readlink -e "${TMP_ROOT}" 2> /dev/null || perl -MCwd -e 'print Cwd::abs_path shift' "${TMP_ROOT}")

function clean {
    rm -rf "${TEMP_DIR}"
    echo "Deleted working dir ${TEMP_DIR}"
}

source "${REPO_ROOT}"/hack/config
source "${REPO_ROOT}"/hack/defaults

trap clean EXIT

for manifest in $OPERATOR_MANIFESTS; do
    echo "${manifest}"
    wget -P "${TEMP_DIR}" "${manifest}"
done

kubevirt_sed
cdi_sed
echo "Replaced image strings"

oc create -f ${TEMP_DIR}/
