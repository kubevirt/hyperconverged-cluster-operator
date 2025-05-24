#!/usr/bin/env bash

set -xe

ASSETS_DIR=${ASSETS_DIR:-/assets}
DICTS_DIR="${ASSETS_DIR}/dataImportCronTemplates"
IS_DIR="${ASSETS_DIR}/imageStreams"

if [[ ! -d "${DICTS_DIR}" ]]; then
  echo "ERROR: Directory ${DICTS_DIR} does not exist. Exiting."
  exit 1
fi

if [[ -d "${IS_DIR}" ]]; then
  IS_PARAM="--image-stream-dir=${IS_DIR}"
fi

annotate-dicts -i --dict-dir="${DICTS_DIR}" ${IS_PARAM}
