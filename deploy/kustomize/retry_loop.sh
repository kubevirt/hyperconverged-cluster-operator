#!/bin/bash

set -x

# Deploy HCO and OLM Resources with retries
retry_loop() {
  success=0
  iterations=0
  sleep_time=10
  max_iterations=72 # results in 12 minutes timeout
  until [[ $success -eq 1 ]] || [[ $iterations -eq $max_iterations ]]
  do
    deployment_failed=0

      if [[ ! -d $1 ]]; then
        echo $1
        echo "[ERROR] Manifests do not exist. Aborting..."
        exit 1
      fi

      set +e
      if ! ${OC_TOOL} apply -k $1
      then
        deployment_failed=1
      fi
      set -e

    if [[ deployment_failed -eq 1 ]]; then
      iterations=$((iterations + 1))
      iterations_left=$((max_iterations - iterations))
      echo "[WARN] At least one deployment failed, retrying in $sleep_time sec, $iterations_left retries left"
      sleep $sleep_time
      continue
    fi
    success=1
  done

  if [[ $success -eq 1 ]]; then
    echo "[INFO] Deployment successful, waiting for HCO Operator to report Ready..."
    ${OC_TOOL} wait -n ${TARGET_NAMESPACE} hyperconverged kubevirt-hyperconverged --for condition=Available --timeout=15m
    ${OC_TOOL} wait "$(${OC_TOOL} get pods -n ${TARGET_NAMESPACE} -l name=hyperconverged-cluster-operator -o name)" -n "${TARGET_NAMESPACE}" --for condition=Ready --timeout=15m
  else
    echo "[ERROR] Deployment failed."
    exit 1
  fi
}