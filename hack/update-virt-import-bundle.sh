#!/usr/bin/env bash
# Downloads the install-hco.yaml release asset of kubevirt/virt-import for VIRT_IMPORT_VERSION
# from hack/config, verifies it against the release's CHECKSUMS.sha256 and updates the bundle
# HCO embeds.
set -ex

BUNDLE_FILE="pkg/virtimport/data/install-hco.yaml"

source hack/config

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

gh release download "${VIRT_IMPORT_VERSION}" \
  -R kubevirt/virt-import \
  -D "${TMP_DIR}" \
  --pattern=install-hco.yaml \
  --pattern=CHECKSUMS.sha256 \
  --clobber

(cd "${TMP_DIR}" && grep ' install-hco.yaml$' CHECKSUMS.sha256 | sha256sum --check --strict)

if ! diff -q "${BUNDLE_FILE}" "${TMP_DIR}/install-hco.yaml" > /dev/null; then
  mv "${TMP_DIR}/install-hco.yaml" "${BUNDLE_FILE}"
fi
