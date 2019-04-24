#!/bin/bash

set -ex

# Create the namespace for the HCO
oc create ns kubevirt-hyperconverged || true

# Create an OperatorGroup
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: hco-operatorgroup
  namespace: kubevirt-hyperconverged
EOF

# Create a Catalog Source backed by a grpc registry
#
# Notes for QE:
#   - This is going to be replaced by https://quay.io/application/redhat-operators-stage/kubevirt-hyperconverged in the future
#
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: hco-catalogsource
  namespace: openshift-operator-lifecycle-manager
  imagePullPolicy: Always
spec:
  sourceType: grpc
  image: docker.io/rthallisey/hco-registry:qe-1.6
  displayName: KubeVirt HyperConverged
  publisher: Red Hat
EOF

WAIT_TIMEOUT="360s"
echo "Waiting up to ${WAIT_TIMEOUT} for catalogsource to appear..."
oc wait pod $(oc get pods -n openshift-operator-lifecycle-manager | grep hco-catalogsource | awk '{print $1}') --for condition=Ready -n openshift-operator-lifecycle-manager --timeout="${WAIT_TIMEOUT}"

# Create a subscription
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: hco-subscription
  namespace: kubevirt-hyperconverged
spec:
  channel: alpha
  name: kubevirt-hyperconverged
  source: hco-catalogsource
  sourceNamespace: openshift-operator-lifecycle-manager
EOF

echo "Give the operators some time to start..."
sleep 30

VIRT_POD=`oc get pods -n kubevirt-hyperconverged | grep virt-operator | head -1 | awk '{ print $1 }'`
CDI_POD=`oc get pods -n kubevirt-hyperconverged | grep cdi-operator | head -1 | awk '{ print $1 }'`
NETWORK_ADDONS_POD=`oc get pods -n kubevirt-hyperconverged | grep cluster-network-addons-operator | head -1 | awk '{ print $1 }'`
oc wait pod $VIRT_POD --for condition=Ready -n kubevirt-hyperconverged --timeout="${WAIT_TIMEOUT}"
oc wait pod $CDI_POD --for condition=Ready -n kubevirt-hyperconverged --timeout="${WAIT_TIMEOUT}"
oc wait pod $NETWORK_ADDONS_POD --for condition=Ready -n kubevirt-hyperconverged --timeout="${WAIT_TIMEOUT}"

echo "Launching CNV..."
cat <<EOF | oc create -f -
apiVersion: hco.kubevirt.io/v1alpha1
kind: HyperConverged
metadata:
  name: hyperconverged-cluster
  namespace: kubevirt-hyperconverged
EOF

