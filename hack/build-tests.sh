#!/bin/bash

set -euo pipefail

TEST_DIRECTORY="${NAME:-func-tests}"

go get github.com/mattn/goveralls
go get -v github.com/onsi/ginkgo/ginkgo
go get -v github.com/onsi/gomega
export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"

if [ "${JOB_TYPE}" == "travis" ]; then
    go get -v -t ./...
    go get -u github.com/evanphx/json-patch
    PACKAGE_PATH="pkg/controller/hyperconverged/"
    ginkgo -r -cover ${PACKAGE_PATH}
else 
    test_path="tests/${TEST_DIRECTORY}"
    test_out_path=${test_path}/_out
    mkdir -p ${test_out_path}
    ginkgo build ${test_path}
    mv ${test_path}/${TEST_DIRECTORY}.test ${test_out_path}
fi
