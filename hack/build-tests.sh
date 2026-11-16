#!/bin/bash

set -euxo pipefail

export PATH=$PATH:$HOME/gopath/bin
set +u
test_path="./tests/func-tests"
test_out_path=${test_path}/_out
mkdir -p ${test_out_path}

if [[ -n ${ARCH} ]]; then
  export GOARCH="${ARCH}"
fi
go tool ginkgo build -o ${test_out_path} ${test_path}
