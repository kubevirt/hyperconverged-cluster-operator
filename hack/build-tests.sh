#!/bin/bash

set -euo pipefail

export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"

if [ "${JOB_TYPE}" == "travis" ]; then
    go mod tidy
    go install github.com/mattn/goveralls@latest
    go install github.com/onsi/ginkgo/v2/ginkgo@latest
    mkdir -p coverprofiles
    # Workaround - run tests on webhooks first to prevent failure when running all the test in the following line.
    ginkgo run -cover -output-dir=./coverprofiles -coverprofile=cover.coverprofile -procs=1 \
      ./controllers/util \
      ./controllers/common \
      ./controllers/operands \
      ./controllers/webhooks \
      ./controllers/hyperconverged
else
    test_path="tests/func-tests"
    (cd $test_path; GOFLAGS='' go install github.com/onsi/ginkgo/v2/ginkgo@latest)
    (cd $test_path; go mod tidy; go mod vendor)
    test_out_path=${test_path}/_out
    mkdir -p ${test_out_path}
    (cd $test_path; ginkgo build .)
    mv ${test_path}/func-tests.test ${test_out_path}
fi
