#!/usr/bin/env bash

source hack/common.sh

set -ex

CERT_MANAGER_TIMEOUT=${CERT_MANAGER_TIMEOUT:-"120s"}

echo "Installing cert-manager ${CERT_MANAGER_VERSION}"
${CMD} apply -f https://github.com/cert-manager/cert-manager/releases/download/${CERT_MANAGER_VERSION}/cert-manager.yaml

echo "Waiting for cert-manager to be ready..."
${CMD} wait --for=condition=Available --namespace=cert-manager deployment/cert-manager --timeout=${CERT_MANAGER_TIMEOUT}
${CMD} wait --for=condition=Available --namespace=cert-manager deployment/cert-manager-cainjector --timeout=${CERT_MANAGER_TIMEOUT}
${CMD} wait --for=condition=Available --namespace=cert-manager deployment/cert-manager-webhook --timeout=${CERT_MANAGER_TIMEOUT}

echo "cert-manager ${CERT_MANAGER_VERSION} installed successfully"
