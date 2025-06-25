#!/bin/bash

set -euxo pipefail

export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"

if [ "${JOB_TYPE}" == "travis" ]; then
    go get -v -t ./...
    go install github.com/mattn/goveralls@latest
    go mod vendor
    PACKAGES=($(go list ./pkg/... ./controllers/... | grep -v 'controllers/commontestutils'))
    COVERPKG="$(echo ${PACKAGES[@]}|sed 's| |,|g')"
    mkdir -p coverprofiles
    # Workaround - run tests on webhooks first to prevent failure when running all the test in the following line.
    go test -v -outputdir=./coverprofiles -coverpkg="${COVERPKG}" -coverprofile=cover.coverprofile ${PACKAGES[@]}

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
