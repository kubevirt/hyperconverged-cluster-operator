#!/bin/bash

set -e

# Create the namespace for the HCO
oc create ns kubevirt-hyperconverged

# Create an OperatorGroup
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: hco-operatorgroup
  namespace: kubevirt-hyperconverged
EOF

# Create a Catalog Source
cat <<EOF | oc create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: hco-catalogsource
  namespace: openshift-operator-lifecycle-manager
spec:
  sourceType: grpc
  image: brew-pulp-docker01.web.prod.ext.phx2.redhat.com:8888/container-native-virtualization/hyperconverged-cluster-operator:v2.0.0-4
  displayName: KubeVirt HyperConverged
  publisher: Red Hat
EOF

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
