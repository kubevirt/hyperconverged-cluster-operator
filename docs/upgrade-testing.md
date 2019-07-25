# Testing Upgrades
This document describes how upgrades are tested upstream.

## Versions
* Current Release - The last published release.
* Next Release (Master + PR) - This the set of changes that have been merged 
since the last release and includes changes introduced by the PR.

When a new release is published, a CSV for the next release is also created. This
new CSV is what will be published as the next version and upgrade path during testing. 

## OLM channel configuration
An upgrade will be performed through a channel change. In the olm-catalog package.yaml,
the current release is configured as channel-v1. The next release is configured as
channel-v2.

## Test
* The build scripts creates a new operator image for "Master + PR". The image
is pushed to the local registry.
* The build scripts creates a new registry image that includes both the current
release version and the "Master + PR" version. The registry image is pushed
to the local registry.
* The HCO is initially installed using the current release using the publish CSV. 
* The steps above is performed by ./cluster/operator-push.sh.
* The test validates that all resources are created, are in a good state, and
are using the correct images for the current release.
* Next an upgrade to the next release, "Master + PR", is performed. The upgrade is 
initiated by updating the subscription by updating the channel from channel-v1
to channel-v2.
* The test validates that all resources have been upgraded if necessary, are in a 
good state, and are using an updated image for the next release if a change was 
made. At minimum we should be able to validate a change in the HCO operator image,
if none of the sub-component images have chagned.

## Commands
* export KUBEVIRT_PROVIDER=okd-4.1.0 (okd-4.1.0 is not supported in the HCO repo, will need an update)
* make cluster-up
* make test/e2e/lifecycle
* make cluster-down

## TODO
* ATM, much of the code is a copy of the testing framework from cluster-network-addons-operator. You 
will see alot of their code commented out. These will need to be converted over to HCO once the
tests are fleshed out.
* Update tests to install the current release.
* Update tests to perform upgrade.
* Update tests to do validation of each release.
* Update our kubevirtci infrastructure to use okd-4.1.x. Otherwise ./cluster/operator-push.sh fails
when it attempts to create the operator group, catalog source, etc.. because current k8s and os-3.11
doesn't support the required APIs.
