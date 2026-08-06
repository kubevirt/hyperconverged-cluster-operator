#!/bin/bash

set -euxo pipefail

export PATH=$PATH:$HOME/gopath/bin
set +u
test_path="./tests/func-tests"
GOFLAGS='' go install "github.com/onsi/ginkgo/v2/ginkgo@$(grep github.com/onsi/ginkgo go.mod | cut -d " " -f2)"
test_out_path=${test_path}/_out
mkdir -p ${test_out_path}

if [[ -n ${ARCH} ]]; then
  export GOARCH="${ARCH}"
fi
ginkgo build -o ${test_out_path} ${test_path}
