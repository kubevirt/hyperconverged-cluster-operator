#!/bin/bash

set -euxo pipefail

export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"

if [ "${JOB_TYPE}" == "travis" ]; then
  mkdir -p coverprofiles
  go test -v -outputdir=./coverprofiles \
     -coverpkg=./api/v1beta1,./pkg/...,./controllers/... \
     -coverprofile=cover.coverprofile.temp \
     ./api/v1beta1 ./pkg/... ./controllers/...
  # don't compute coverage of auto generated code
  grep -v zz_generated ./coverprofiles/cover.coverprofile.temp > ./coverprofiles/cover.coverprofile
  rm ./coverprofiles/cover.coverprofile.temp
else
    set +u
    test_path="./tests/func-tests"
    GOFLAGS='' go install github.com/onsi/ginkgo/v2/ginkgo@$(grep github.com/onsi/ginkgo go.mod | cut -d " " -f2)
    go mod tidy
    go mod vendor
    test_out_path=${test_path}/_out
    mkdir -p ${test_out_path}

    if [[ -n ${ARCH} ]]; then
      export GOARCH="${ARCH}"
    fi
    ginkgo build -o ${test_out_path} ${test_path}
fi
