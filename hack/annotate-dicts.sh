#!/usr/bin/env bash

# This script annotates the DataImportCronTemplate objects in the dataImportCronTemplates.yaml file with the
# specific image supported architectures.

set -ex

ASSETS_DIR=${ASSETS_DIR:-assets}
DICTS_DIR="${ASSETS_DIR}/dataImportCronTemplates"
IS_DIR="${ASSETS_DIR}/imageStreams"

if [[ ! -d "${DICTS_DIR}" ]]; then
  echo "ERROR: Directory ${DICTS_DIR} does not exist. Exiting."
  exit 1
fi

go build -o _out/annotate-dicts ./tools/annotate-dicts/

if [[ -d "${IS_DIR}" ]]; then
  IS_PARAM="--image-stream-dir=${IS_DIR}"
fi

_out/annotate-dicts -i --dict-dir="${DICTS_DIR}" ${IS_PARAM}
