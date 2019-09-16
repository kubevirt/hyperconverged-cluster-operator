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

oc patch configs.imageregistry.operator.openshift.io/cluster --type merge -p '{"spec":{"defaultRoute":true}}'

oc get routes -n openshift-image-registry -o yaml 

REGISTRY_URL=`oc get routes -n openshift-image-registry | grep image-registry | tr -s ' ' | cut -d ' ' -f 2`

echo $REGISTRY_URL

dnf install podman -y
sudo dnf install podman -y

oc sa get-token -n openshift builder | podman login -u builder --tls-verify=false --password-stdin $REGISTRY_URL

#BUILDER_TOKEN=`oc sa get-token -n openshift builder`
#oc login --token="$BUILDER_TOKEN"

oc create ns kubevirt | true
export CONTAINER_TAG=upgrade
export REGISTRY_NAMESPACE=kubevirt
export IMAGE_REGISTRY=$REGISTRY_URL
export CONTAINER_BUILD_CMD="podman"
make bundleRegistry

jq

docker version

rpm -qa | grep docker