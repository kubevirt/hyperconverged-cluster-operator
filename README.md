OLM Manifests
=============

Integration point for all the components of KubeVirt's OLM related manifests.

# Cluster Requirements

The [`registry`](registry/) directory makes use of OLM's bundle support that is
provided only via the `grpc` type `CatalogSource`. You can see us create this
`CatalogSource` in the provided [`playbook.yaml`](playbook.yaml). This means
that you **must** have a relatively new **OpenShift 4.0** cluster.

# Ansible Requirements

If you want to run the playbook you will need a recent version of Ansible (`>=
2.6`) and the [`openshift`
client](https://github.com/openshift/openshift-restclient-python/).

# Example Run

1. Get a running OpenShift 4.0 Cluster.
1. Build the registry image: `docker build -t docker.io/djzager/kubevirt-operators:$(git rev-parse --short HEAD) -f Dockerfile .`
1. Push the image: `docker push docker.io/djzager/kubevirt-operators:$(git rev-parse --short HEAD)`
1. Run the Playbook `ansible-playbook playbook.yaml -e registry_image=$(git rev-parse --short HEAD)`
1. The KubevirtGroup, KubeVirt, and CDI Operators should be visible in the UI.
1. You **MUST** create the subscription in the namespace used in the
   [`playbook.yaml`](playbook.yaml).

## Notes

* You must specify the `registry_image` when calling the playbook. I just chose
    not to have defaults.
* Assuming you have cleaned up all of the subscriptions and CRs that you created
  you can run `ansible-playbook playbook.yaml -e state=absent` to delete
  the things created by the playbook. This should make it easy to iterate but
  **requires** you to clean up all of the stuff not being managed by either OLM or the
  playbook (think a KubeVirt CustomRESOURCE).

# Some references

1. Information about the operator-registry image that the `kubevirt-operators` image is
   based on https://github.com/operator-framework/operator-registry/
1. Information about how to build a CSV
   https://github.com/operator-framework/operator-lifecycle-manager/blob/master/Documentation/design/building-your-csv.md
1. The community operators project
   https://github.com/operator-framework/community-operators/
