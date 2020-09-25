#!/bin/bash

set -euo pipefail

export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"

if [ "${JOB_TYPE}" == "travis" ]; then
    go get -v -t ./...
    go get github.com/mattn/goveralls
    go get -v github.com/onsi/ginkgo/ginkgo
    go get -v github.com/onsi/gomega
    go get -u github.com/evanphx/json-patch
    go mod vendor
    PACKAGE_PATH="pkg/"
    ginkgo -r -cover ${PACKAGE_PATH}
else
    GOFLAGS= go get github.com/onsi/ginkgo/ginkgo
    GOFLAGS= go get github.com/onsi/gomega/...
    test_path="tests/func-tests"
    test_out_path=${test_path}/_out
    mkdir -p ${test_out_path}
    ginkgo build ${test_path}
    mv ${test_path}/func-tests.test ${test_out_path}
fi
