#!/usr/bin/env bash

set -ex

KUBEVIRTCI_TAG=$(curl -L -Ss https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest)
echo "${KUBEVIRTCI_TAG}" > cluster/kubevirtci_tag.txt

