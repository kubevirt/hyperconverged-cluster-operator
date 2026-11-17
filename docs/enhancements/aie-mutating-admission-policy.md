# AIE launcher selection through MutatingAdmissionPolicy

## Summary

Replace the standalone AIE webhook with an HCO-managed
MutatingAdmissionPolicy and binding. Enabling AIE selects the bundle-provided
NVIDIA launcher for every newly created virt-launcher Pod across workload
namespaces. The CNV v5.1 delivery work is tracked by
[CNV-98728](https://redhat.atlassian.net/browse/CNV-98728).

## Motivation

Launcher selection needs no separate server, serving certificate, webhook
TLS profile, service, or NetworkPolicy. Removing that operand also removes
the plugin-based replacement dependency and webhook readiness condition.

## Goals

- Introduce the disabled-by-default alpha `nvidiaAIE` feature gate and remove
  support for the former Developer Preview `hco.kubevirt.io/deployAIE` annotation.
- Select the NVIDIA launcher from the released bundle, without a hardcoded
  image or customer-managed image-selection ConfigMap.
- Preserve Pod sidecars, container arguments, resources, and VM placement.
- Reconcile drift, image changes, disabling AIE, upgrades and uninstall.
- Remove the retired operand without leaving a fail-closed admission
  registration pointing at a deleted server.

## Non-goals

- Per-VM selection, ConfigMap rules, or migration of old rule selectors.
- Injecting node affinity from the old webhook configuration. Placement
  belongs on the VM; transfer required rule-based placement before upgrade.
- Changing GPU provisioning or native IOMMUFD allocation.
- Restarting existing VMs or qualifying a CNV/RHEL4NV GPU stack.

## Proposal

HCO installs `kubevirt-aie-launcher` and its `-binding` when `nvidiaAIE` is
enabled. The former annotation is ignored; users must configure the v1 gate
when upgrading to enable the integration. The same gate enables the KubeVirt `GraceIOVirtualization`,
`IOMMUFD`, and `PCINUMAAwareTopology` gates.

The policy matches CREATE requests for core/v1 Pods
labeled `kubevirt.io: virt-launcher`, with a controlling KubeVirt v1 VMI
owner reference and a `compute` container. It applies across namespaces.

An ApplyConfiguration mutation updates the `compute` container by name and
sets `kubevirt.io/alternative-launcher-image` on the Pod. Kubernetes merges
these fields with the admission object, preserving other containers and
fields. Reinvocation is `IfNeeded`; applying the desired image twice is
idempotent. Evaluation failures use `failurePolicy: Fail`.

The policy cannot look up a VMI or choose a rule based on its devices. That
lookup is unnecessary for the agreed installation-wide selection.

### Image delivery

The manifest tools accept `--aie-launcher-image-name`, which populates
`AIE_LAUNCHER_IMAGE` in both the controller and the HCO validating webhook.
Empty image inputs omit the flag and environment variable. The policy embeds
the exact supplied image reference, prepared once when its handler is built.

The intended community default is a release-matched image from
`quay.io/kubevirt/kubevirt-aie/virt-launcher`, tracked through
`deploy/images.csv` and the standard digest generator. A matching v1.10 image
must be published and pinned before this default can be enabled in the image
catalogue. Downstream bundles may override it with their product image; their
packaging must retain that image in relatedImages/release inputs.

The community publisher in [project-infra #5639](https://github.com/kubevirt/project-infra/pull/5639)
uses release/date/commit tags such as `1.10-aie-nv_YYYYMMDD_<commit>`.
The catalogue integration should pin a published tag through a dedicated
`AIE_LAUNCHER_VERSION` input. The existing digest updater refreshes that
configured tag; advancing to a newer stamped tag requires a version bump.
A floating tag is not required. The AIE image uses the relatedImages name
`aie-virt-launcher` to distinguish it from the standard launcher.

Without a nonempty image, HCO logs that NVIDIA AIE is unavailable and omits
its policy/binding handlers. The validating webhook rejects creation or
enablement updates requesting NVIDIA AIE without an image, through either API
version. Already-enabled objects may still change unrelated configuration or
disable the integration. A missing image does not fail overall reconciliation.
The former AIE webhook image flag remains accepted and ignored during migration.

### Reconciliation and uninstall

Policy and binding use HCO conditional operands, labels, relatedObjects and
normal uninstall tracking. Both resource types are watched so edits or
deletions trigger reconciliation. Disabling the effective AIE setting removes
them. A v1-only gate survives round trips through the frozen v1beta1 schema
using the existing conversion preservation mechanism.

### Upgrade cleanup

The existing HCO `objectsToBeRemoved` upgrade mechanism removes the legacy
MutatingWebhookConfiguration, Deployment, Service, ServiceAccount, ClusterRole,
ClusterRoleBinding, named NetworkPolicy, serving Secret and rule ConfigMap for
source HCO versions `<1.20.0`. It removes their relatedObjects references;
the upgrade path separately removes `KubevirtAieWebhookReady` from status.
The admission registration is listed before its former server resources.

This follows standard HCO cleanup behavior, including its version matching,
explicit object identities and deletion semantics. It does not retain a
dedicated legacy operand or label-based discovery of alternate resource names.
Cleanup runs before normal operand reconciliation. An interruption in image
selection between webhook removal and activation of the policy is accepted.
The change does not promise atomic or measured zero-gap admission cutover.

## Testing

The tests cover feature-gate enablement and ignoring retired annotations, policy drift,
image updates after handler recreation, disable/uninstall cleanup, v1beta1
conversion preservation, image availability at startup, create/update admission
through both API versions, and standard version-scoped upgrade cleanup.
Manifest tests cover image delivery to both HCO deployments and omission of
empty inputs; the unit script includes `tools/manifests`, while coverage
remains limited to the deployed components.

The functional test enables `nvidiaAIE`, exercises API-server dry-run mutation,
checks policy creation/removal and checks absence of the retired registration
and Deployment. It skips installations without an AIE launcher image.

The November 13 draft was exercised against an isolated Kubernetes v1.37.0 API
server, including named compute-container mutation, preservation of other Pod
fields, missing annotations, nonmatching Pods and idempotence. These dated
checks do not establish install/upgrade or target GPU-stack qualification.

On November 16, full `make build-manifests` generation reproduced the
committed baseline without changes. A second full generation using the
published v1.9 bootstrap image as a rendering fixture verified delivery to
both HCO deployments and both CSVs, exact retention in relatedImages, and
distinct image names for the standard and AIE launchers. The image was only
embedded in generated manifests in an isolated checkout; no GPU VM was run,
and this does not qualify the pending v1.10 NVIDIA launcher.

## Release qualification

[CNV-100753](https://redhat.atlassian.net/browse/CNV-100753) still owns
fresh CNV v5.1 install and v5.0-to-v5.1 upgrade execution, real GPU VM
startup, configured node affinity and native IOMMUFD access. Qualification
still requires the pinned v1.10 AIE bundle and downstream product image.
The generation checks above do not establish runtime qualification.

## References

- [Implementation story](https://redhat.atlassian.net/browse/CNV-98730)
- [Retirement epic](https://redhat.atlassian.net/browse/CNV-98728)
- [Kubernetes MutatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/)
