#!/usr/bin/env bash

set -euo pipefail

source hack/common.sh

if [ "${JOB_TYPE}" == "stdci" ]; then
  echo "Running on STDCI"
  KUBECONFIG=${KUBEVIRTCI_PATH}/$KUBEVIRT_PROVIDER/.kubeconfig
  source ./hack/upgrade-stdci-config
else
  echo "Running on OpenShift CI"
  set +u
  set +x
  source ./hack/upgrade-openshiftci-config
  set -u
fi
 
# check if CSV test is requested (if this is run right after upgrade-test.sh)
CSV_FILE=./test-out/clusterserviceversion.yaml 
if [ -f ${CSV_FILE} ]; then
    echo "** enable CSV test **"
    export TEST_KUBECTL_CMD="${CMD}"
    export TEST_CSV_FILE="${CSV_FILE}"
fi

./${TEST_OUT_PATH}/func-tests.test -ginkgo.v -test.timeout 120m -kubeconfig="${KUBECONFIG}" 

if [ -f ${CSV_FILE} ]; then
  rm -f ${CSV_FILE}
fi  
