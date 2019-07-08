  #!/bin/bash -e

registry_port=$(./cluster/cli.sh ports registry | tr -d '\r')
registry=localhost:$registry_port

echo "INFO: registry: $registry"

make cluster-clean

IMAGE_REGISTRY=$registry make docker-build-operator docker-push-operator docker-build-registry docker-push-registry

# check images are accessible
for i in $(seq 1 ${CLUSTER_NUM_NODES}); do
    ./cluster/cli.sh ssh "node$(printf "%02d" ${i})" 'sudo docker pull registry:5000/kubevirt/hyperconverged-cluster-operator'
    ./cluster/cli.sh ssh "node$(printf "%02d" ${i})" 'sudo docker pull registry:5000/kubevirt/hyperconverged-cluster-operator-registry'
    # Temporary until image is updated with provisioner that sets this field
    # This field is required by buildah tool
    ./cluster/cli.sh ssh "node$(printf "%02d" ${i})" 'sudo sysctl -w user.max_user_namespaces=1024'
done

./cluster/kubectl.sh create ns kubevirt-hyperconverged

cat <<EOF | ./cluster/kubectl.sh create -f -
apiVersion: operators.coreos.com/v1alpha2
kind: OperatorGroup
metadata:
  name: hco-operatorgroup
  namespace: kubevirt-hyperconverged
EOF

cat <<EOF | ./cluster/kubectl.sh create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: hco-catalogsource-example
  namespace: openshift-operator-lifecycle-manager
spec:
  sourceType: grpc
  image: registry:5000/kubevirt/hyperconverged-cluster-operator-registry
  displayName: KubeVirt HyperConverged
  publisher: Red Hat
EOF

cat <<EOF | ./cluster/kubectl.sh create -f -
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: hco-subscription-example
  namespace: kubevirt-hyperconverged
spec:
  channel: alpha
  name: kubevirt-hyperconverged
  source: hco-catalogsource-example
  sourceNamespace: openshift-operator-lifecycle-manager
EOF

./cluster/kubectl.sh create -f ./deploy/converged/crds/hco.cr.yaml -n kubevirt-hyperconverged
