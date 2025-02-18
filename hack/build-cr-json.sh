#!/usr/bin/env bash
set -ex

if [[ -n ${BUILDOS} ]]; then
  export GOOS=${BUILDOS}
fi

if [[ -n ${BUILDARCH} ]]; then
  export GOARCH=${BUILDARCH}
fi

PROJECT_ROOT="$(readlink -e $(dirname "$BASH_SOURCE[0]")/../)"

go build -o _out/crwriter ${PROJECT_ROOT}/tools/crwriter

./_out/crwriter --format=json --out=$1
