#!/usr/bin/env bash

ARCHITECTURES="amd64 arm64 s390x"

if [[ -z ${IMAGE_REG} ]]; then
  echo "IMAGE_NAME must be defined"
  exit 1
fi

NEW_IMAGE_REG=${NEW_IMAGE_REG:-${IMAGE_REG}}

if [[ -z ${CURRENT_TAG} ]]; then
  echo "CURRENT_TAG must be defined"
  exit 1
fi

if [[ -z ${NEW_TAG} ]]; then
  echo "NEW_TAG must be defined"
  exit 1
fi

if [[ "${MULTIARCH}" == "true" ]]; then
  for arch in ${ARCHITECTURES}; do
    NEW_IMAGE="${NEW_IMAGE_REG}:${NEW_TAG}-${arch}"
    . "hack/cri-bin.sh" && ${CRI_BIN} tag "${IMAGE_REG}:${CURRENT_TAG}-${arch}" "${NEW_IMAGE}"
    . "hack/cri-bin.sh" && ${CRI_BIN} push "${NEW_IMAGE}"
  done
fi

# retug the manifest
NEW_IMAGE="${NEW_IMAGE_REG}:${NEW_TAG}"
. "hack/cri-bin.sh" && ${CRI_BIN} tag "${IMAGE_REG}:${CURRENT_TAG}" "${NEW_IMAGE}"
. "hack/cri-bin.sh" && ${CRI_BIN} push "${NEW_IMAGE}"
