# Running the upgrade test on a Code Ready Containers deployed cluster

This document describes how to create and setup an environment to
run the HCO upgrade test against a CRC deployed OpenShift cluster.

## Deploy a OpenShift cluster using Code Ready Containers

Deploy a OpenShift cluster using the CRC instructions: https://code-ready.github.io/crc/.

## Login to the cluster as an admin

````
eval $(crc oc-env)
oc login -u kubeadmin -p <replace-with-your-password> https://api.crc.testing:6443
````

Use the kubeadmin password that is printed to STDOUT, after "crc start" completes.

## Login to the cluster's image registry

````
podman login -u kubeadmin -p $(oc whoami -t) --tls-verify=false default-route-openshift-image-registry.apps-crc.testing

````

## Create the kubevirt namespace

The HCO operator and registry images are built and pushed to your cluster's image registry under the "kubevirt" namespace.
Create this namespace before upgrade test builds and pushes the images.

````
oc create ns kubevirt
````

## Create ClusterRoleBindings

Create ClusterRoleBindings to allow pods to read from the image registry.

````
cat <<EOF | oc create -f -
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: hco-marketplace-registry-viewer-role-binding-cluster-wide
subjects:
  - kind: ServiceAccount
    name: default
    namespace: openshift-marketplace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: registry-viewer
EOF

cat <<EOF | oc create -f -
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kubevirt-registry-viewer-role-binding-cluster-wide
subjects:
  - kind: ServiceAccount
    name: hyperconverged-cluster-operator
    namespace: kubevirt-hyperconverged
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: registry-viewer
EOF
````

## Install podman-docker

If docker isn't already installed.

````
sudo dnf install podman-docker
````

## Run the upgrade test

Pass in a value for the FOR_CRC environment variable and call "make upgrade-test" to run the upgrade test. Setting a value for FOR_CRC indicates to the upgrade test to use settings to run against a CRC cluster.

KUBECONFIG path may need to be adjusted depending on your CRC version.

````
export KUBECONFIG=~/.crc/cache/crc_libvirt_4.3.0/kubeconfig
FOR_CRC=true make upgrade-test
````