#!/bin/bash

set -ex

APP_REGISTRY="${APP_REGISTRY:-redhat-operators}"
PACKAGE="${PACKAGE:-kubevirt-hyperconverged}"
OPERATOR="hyperconverged-cluster-operator"
TARGET_NAMESPACE="${TARGET_NAMESPACE:-openshift-cnv}"
CLUSTER="${CLUSTER:-OPENSHIFT}"
MARKETPLACE_NAMESPACE="${MARKETPLACE_NAMESPACE:-openshift-marketplace}"
HCO_VERSION="${HCO_VERSION:-2.1.0}"
HCO_CHANNEL="${HCO_CHANNEL:-2.1}"

if [ "${CLUSTER}" == "KUBERNETES" ]; then
    MARKETPLACE_NAMESPACE="marketplace"
    OPERATOR="hco-operator"
    APP_REGISTRY="kubevirt-hyperconverged"
    HCO_VERSION="${HCO_VERSION:-1.0.0}"
    HCO_CHANNEL="${HCO_CHANNEL:-1.0.0}"
    TARGET_NAMESPACE="${TARGET_NAMESPACE:-kubevirt-hyperconverged}"    
fi


oc create ns $TARGET_NAMESPACE || true

oc -n "${TARGET_NAMESPACE}" delete og   ${PACKAGE}-operatorgroup  || true
oc -n "${TARGET_NAMESPACE}" delete sub  ${PACKAGE}-subscription   || true
oc -n "${TARGET_NAMESPACE}" delete csv  --all                     || true
oc -n "${TARGET_NAMESPACE}" delete ip   --all                     || true

cat << __EOF__ | oc create -f -
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: "${PACKAGE}-operatorgroup"
  namespace: "${TARGET_NAMESPACE}"
spec:
  serviceAccount:
    metadata:
      creationTimestamp: null
  targetNamespaces:
  - "${TARGET_NAMESPACE}"
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: "${PACKAGE}-subscription"
  namespace: "${TARGET_NAMESPACE}"
spec:
  sourceNamespace: "${MARKETPLACE_NAMESPACE}"
  source: "${APP_REGISTRY}"  
  name: "${PACKAGE}"
  channel: "${HCO_CHANNEL}"  
  startingCSV: "${PACKAGE}-operator.v${HCO_VERSION}"  
__EOF__
#wait for it
retries=0; while [ $retries -lt 200 ] && ! oc wait --for condition=ready pod -l name=${OPERATOR} -n ${TARGET_NAMESPACE} --timeout=1m ; do sleep 15; let retries=$retries+1; done


oc -n "${TARGET_NAMESPACE}" delete hco --all || true

cat << __EOF__ | oc create -f -
apiVersion: hco.kubevirt.io/v1alpha1
kind: HyperConverged
metadata:
  name: hyperconverged-cluster
  namespace: "${TARGET_NAMESPACE}"
spec:
  BareMetalPlatform: true
__EOF__
#wait for it
retries=0; while [ $retries -lt 200 ] && ! oc wait --for condition=Available hyperconverged hyperconverged-cluster -n ${TARGET_NAMESPACE} --timeout=2m ; do sleep 15; let retries=$retries+1; done

