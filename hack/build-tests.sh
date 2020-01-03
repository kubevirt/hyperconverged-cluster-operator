#!/bin/bash

set -euo pipefail

go get github.com/mattn/goveralls
go get -v github.com/onsi/ginkgo/ginkgo
go get -v github.com/onsi/gomega
export PATH=$PATH:$HOME/gopath/bin
JOB_TYPE="${JOB_TYPE:-}"
CLEANUP_SERVICE_ACCOUNT_DIR=""

secrets_dir="/var/run/secrets"
service_account_dir="${secrets_dir}/kubernetes.io/serviceaccount"
namespace_path="${service_account_dir}/namespace"
if [ ! -s ${service_account_dir} ]; then
    SUDO=''
    if [ "$EUID" -ne 0 ]; then
        SUDO='sudo'
    fi
    CLEANUP_SERVICE_ACCOUNT_DIR=true
    $SUDO mkdir -p ${service_account_dir}
    echo "kubevirt-hyperconverged" | $SUDO tee ${namespace_path}
fi

if [ "${JOB_TYPE}" == "travis" ]; then
    go get -v -t ./...
    go get -u github.com/evanphx/json-patch
    PACKAGE_PATH="pkg/controller/hyperconverged/"
    ginkgo -r -cover ${PACKAGE_PATH}
else 
    test_path="tests/func-tests"
    test_out_path=${test_path}/_out
    mkdir -p ${test_out_path}
    ginkgo build ${test_path}
    mv ${test_path}/func-tests.test ${test_out_path}
fi

if [ -n ${CLEANUP_SERVICE_ACCOUNT_DIR} ]; then
    sudo rm -r ${secrets_dir}
fi
