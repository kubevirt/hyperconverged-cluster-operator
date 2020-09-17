#!/usr/bin/env bash
set -xe

PROJECT_ROOT="$(readlink -e $(dirname "${BASH_SOURCE[0]}")/../)"
source "${PROJECT_ROOT}/hack/config"
source "${PROJECT_ROOT}/hack/image-names"

DIGESTS_FILE="${PROJECT_ROOT}/hack/digests"

function get_image_digest() {
  local digest
  digest="${1/:*/}@$(docker run --rm quay.io/skopeo/stable:latest inspect "docker://$1" | jq -r '.Digest')"
  echo "${digest}"
}

function create_digest_file() {
  rm "${DIGESTS_FILE}" || true

  touch "${DIGESTS_FILE}"

  local digest

  digest=$(get_image_digest "${KUBEVIRT_IMAGE}")
  echo "KUBEVIRT_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${KUBEVIRT_IMAGE%/*}/virt-api:${KUBEVIRT_IMAGE/*:}")
  echo "KUBEVIRT_API_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${KUBEVIRT_IMAGE%/*}/virt-controller:${KUBEVIRT_IMAGE/*:}")
  echo "KUBEVIRT_CONTROLLER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${KUBEVIRT_IMAGE%/*}/virt-launcher:${KUBEVIRT_IMAGE/*:}")
  echo "KUBEVIRT_LAUNCHER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${KUBEVIRT_IMAGE%/*}/virt-handler:${KUBEVIRT_IMAGE/*:}")
  echo "KUBEVIRT_HANDLER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  digest=$(get_image_digest "${CNA_IMAGE}")
  echo "CNA_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  digest=$(get_image_digest "${SSP_IMAGE}")
  echo "SSP_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  local containerPrefix="${CDI_IMAGE%/*}"
  local cdiTag="${CDI_IMAGE/*:/}"
  digest=$(get_image_digest "${CDI_IMAGE}")
  echo "CDI_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-controller:${cdiTag}")
  echo "CDI_CONTROLLER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-apiserver:${cdiTag}")
  echo "CDI_APISERVER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-cloner:${cdiTag}")
  echo "CDI_CLONER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-importer:${cdiTag}")
  echo "CDI_IMPORTER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-uploadproxy:${cdiTag}")
  echo "CDI_UPLOADPROXY_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${containerPrefix}/cdi-uploadserver:${cdiTag}")
  echo "CDI_UPLOADSERVER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  digest=$(get_image_digest "${HPPO_IMAGE}")
  echo "HPPO_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${HPP_IMAGE}")
  echo "HPP_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  digest=$(get_image_digest "${VM_IMPORT_IMAGE}")
  echo "VM_IMPORT_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${VM_IMPORT_IMAGE%/*}/vm-import-controller:${VM_IMPORT_IMAGE/*:/}")
  echo "VM_IMPORT_CONTROLLER_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  digest=$(get_image_digest "${OPERATOR_IMAGE}")
  echo "HCO_OPERATOR_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${CONVERSION_CONTAINER}")
  echo "CONVERSION_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"
  digest=$(get_image_digest "${VMWARE_CONTAINER}")
  echo "VMWARE_DIGEST=${digest}" >> "${DIGESTS_FILE}"
  DIGEST_LIST="${DIGEST_LIST}${digest},"

  echo "DIGEST_LIST=${DIGEST_LIST}" >> "${DIGESTS_FILE}"
}

if [ ! -f "${DIGESTS_FILE}" ] || ! git diff --quiet --exit-code master ./hack/config; then
  create_digest_file
else
  source "${DIGESTS_FILE}"
  digest=$(get_image_digest "${OPERATOR_IMAGE}")
  if [[ ! "${digest}" == "${HCO_OPERATOR_DIGEST}" ]]; then
    sed -r -i "s|${HCO_OPERATOR_DIGEST}|${digest}|g" "${DIGESTS_FILE}"
  else
    echo no new versions. using cache
  fi
fi
