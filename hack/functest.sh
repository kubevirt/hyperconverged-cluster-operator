#!/bin/bash

#TODO: Launch component e2e tests here

# git clone https://github.com/kubevirt/kubevirt
# make functest . . .

set -x

oc get deployment -n kubevirt-hyperconverged hyperconverged-cluster-operator -o yaml

cat _out/operator.yaml

cat /usr/local/hco-e2e-aws-cluster-profile

cat /usr/local/hco-e2e-aws

set | grep IMAGE_FORMAT

set

ls

oc get routes -n openshift-image-registry -o yaml 

oc get routes -A

cat $KUBECONFIG

