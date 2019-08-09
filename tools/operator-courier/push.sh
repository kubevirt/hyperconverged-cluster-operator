#!/usr/bin/env bash
set -e

PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/../../)"

QUAY_REPOSITORY="${QUAY_REPOSITORY:-kubevirt-hyperconverged}"
PACKAGE_NAME="${PACKAGE_NAME:-kubevirt-hyperconverged}"
SOURCE_DIR="${SOURCE_DIR:-/manifests}"
REPO_DIR="${QUAY_REPOSITORY:-kubevirt-hyperconverged}"
NAMESPACE="${PACKAGE_NAME:-kubevirt-hyperconverged}"

latest_bundle_version=$(curl -X GET https://quay.io/cnr/api/v1/packages/${QUAY_REPOSITORY}/${PACKAGE_NAME} | jq '.[-1]["release"]' | tr -d '"')
RELEASE="${RELEASE:-$latest_bundle_version}"

while [ "${latest_bundle_version}" = "${RELEASE}" ]; do
    echo "Latest bundle version is ${latest_bundle_version}."
    echo "Set RELEASE to a newer version."
    echo "RELEASE"
    read RELEASE
done

if [ -z "${QUAY_USERNAME}" ]; then
    echo "QUAY_USERNAME"
    read QUAY_USERNAME
fi

if [ -z "${QUAY_PASSWORD}" ]; then
    echo "QUAY_PASSWORD"
    read -s QUAY_PASSWORD
fi

echo "getting auth token from Quay"
AUTH_TOKEN=$(/"${PROJECT_ROOT}"/tools/token.sh $QUAY_USERNAME $QUAY_PASSWORD)

echo "pushing bundle"
docker run \
	-e QUAY_USERNAME="${QUAY_USERNAME}" \
	-e QUAY_PASSWORD="${QUAY_PASSWORD}" \
	-e QUAY_REPOSITORY="${REPO_DIR}" \
	hco-courier push "${SOURCE_DIR}" "${REPO_DIR}" "${NAMESPACE}" "${RELEASE}" "$AUTH_TOKEN"
echo "bundle pushed"
