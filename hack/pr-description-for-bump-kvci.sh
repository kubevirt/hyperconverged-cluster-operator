#!/usr/bin/env bash

set -e

KUBEVIRTCI_TAG=$(curl -L -Ss https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest)

echo "Bump kubevirtci to ${KUBEVIRTCI_TAG}"
echo
echo '**Release note**:'
echo '```release-note'
echo 'None'
echo '```'