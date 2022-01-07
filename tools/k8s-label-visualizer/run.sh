#!/bin/bash

set -euox pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARTIFACT_DIR="${ARTIFACT_DIR:-"$DIR/out"}"
NAMESPACE="${NAMESPACE:-"kubevirt-hyperconverged"}"

python3 -mplatform | grep -qEi "Ubuntu|Debian" && apt-get install python3-venv || true

python3 -m venv "${DIR}/venv"
source "${DIR}/venv/bin/activate"
pip3 install -r "${DIR}/requirements.txt"

python3 "${DIR}/main.py" --namespace "${NAMESPACE}" --conf "${DIR}/conf.json" --output "${ARTIFACT_DIR}"
