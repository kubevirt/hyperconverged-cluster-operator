#!/usr/bin/env bash

set -euo pipefail

source ./hack/upgrade-openshiftci-config

export TEST_CSV_FILE=./test-out/clusterserviceversion.yaml
export KUBECONFIG=~/.crc/machines/crc/kubeconfig
export TEST_KUBECTL_CMD=${CMD}

HCO_CATALOGSOURCE_POD=`${CMD} get pods -n ${HCO_CATALOG_NAMESPACE} | grep hco-catalogsource | head -1 | awk '{ print $1 }'`
${CMD} exec -ti -n ${HCO_CATALOG_NAMESPACE} ${HCO_CATALOGSOURCE_POD} cat kubevirt-hyperconverged/100.0.0/kubevirt-hyperconverged-operator.v100.0.0.clusterserviceversion.yaml > $TEST_CSV_FILE

timeout 10m bash -c 'export CMD="${CMD}";exec ./hack/check-state.sh' 

./tests/component-validation-tests/_out/func-tests.test -ginkgo.v -test.timeout 120m -kubeconfig="${KUBECONFIG}" -installed-namespace=kubevirt-hyperconverged

if [ -f ${TEST_CSV_FILE} ]; then
  rm -f ${TEST_CSV_FILE}
fi
