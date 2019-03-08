OLM Manifests
=============

Integration point for all the components of KubeVirt's OLM related manifests.
The goal of this project is to make a KubeVirt package, viewable in OLM, such
that a subscription to KubeVirt provides all of components of KubeVirt.

# Cluster Requirements

The [`registry`](registry/) directory makes use of OLM's bundle support that is
provided only via the `grpc` type `CatalogSource`. You can see us create this
`CatalogSource` in the provided [`playbook.yaml`](playbook.yaml). This means
that you **must** have a relatively new **OpenShift 4.0** cluster.

# Ansible Requirements

If you want to run the playbook you will need a recent version of Ansible (`>=
2.6`) and the [`openshift`
client](https://github.com/openshift/openshift-restclient-python/).

In the case you see an error like:

```
TASK [Kubevirt namespace state=present]
fatal: [localhost]: FAILED! => {"changed": false, "msg": "This module requires
the OpenShift Python client. Try pip install openshift"}
```

The problem is that the python environment in which Ansible is running does not
see the OpenShift Python client. There is some helpful information in this
[Ansible issue comment](https://github.com/ansible/ansible/issues/50529#issuecomment-461894431).

# Example Run

You will first need to clone this repo. The instructions that follow are from the
project's root.

1. Get a running OpenShift 4.0 Cluster.
1. Build the operator-registry image: `docker build -t docker.io/djzager/kubevirt-operators:example -f Dockerfile .`
1. Push the image: `docker push docker.io/djzager/kubevirt-operators:example`
1. Run the Playbook `ansible-playbook playbook.yaml -e registry_image=docker.io/djzager/kubevirt-operators:example`
1. Select the `kubevirt` namespace, this is where we created the OperatorGroup
   in the [playbook.yaml](playbook.yaml).
1. Catalog->Operator Management. The KubeVirt Operator should be visible in the UI.
1. Create the subscription in the `kubevirt` namespace.
1. Navigate to the Workloads->Pods screen for the `kubevirt` namespace all of
   the operators included in the kubevirt CSV should now be running.
1. You will still need to create CRs for those components.

## Notes

* You must specify the `registry_image` when calling the playbook.
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
