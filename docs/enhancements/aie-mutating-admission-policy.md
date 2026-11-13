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

- Preserve the `hco.kubevirt.io/deployAIE: "true"` installation opt-in.
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
- Changing KubeVirt feature gates, GPU provisioning or native IOMMUFD.
- Restarting existing VMs or qualifying a CNV/RHEL4NV GPU stack.

## Proposal

HCO installs `kubevirt-aie-launcher` and its `-binding` when the annotation
is exactly `"true"`. The policy matches CREATE requests for core/v1 Pods
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

The manifest tools accept `--aie-launcher-image-name`. This populates
`AIE_LAUNCHER_IMAGE` on HCO; the policy embeds that exact reference. The
manifest build includes a supplied image in the CSV relatedImages list.
An unset/empty image is valid on installations with AIE disabled. Enabling
AIE without an image fails reconciliation before activating a new policy.
HCO can start without the retired webhook image environment variable.

The CNV v5.1 bundle renderer must pass its catalog-resolved
`virt-launcher-4-nv` digest through the new flag and retain it in
relatedImages/release inputs. This downstream renderer change is a
companion change; it is not implemented in this upstream HCO draft.
The old webhook image flag is accepted and ignored to let callers migrate.

### Reconciliation and uninstall

Policy and binding use HCO conditional operands, labels, relatedObjects and
normal uninstall tracking. Both resource types are watched so edits or
deletions trigger reconciliation. Removing the annotation removes them.

### Upgrade cleanup

The draft creates the policy and binding before retiring the old server.
The cleaner deletes the old MutatingWebhookConfiguration and confirms its
absence before deleting the Deployment, Service, RBAC, ServiceAccount,
managed NetworkPolicies, serving secret and rule ConfigMap. It removes
stale relatedObjects and the `KubevirtAieWebhookReady` condition.

Cleanup runs with AIE enabled or disabled and on uninstall, including when
uninstall is the first reconciliation after upgrading. It uses HCO's
ownership protection and retries errors. If a registration with that name
still exists, its server is retained. Managed policies with the legacy
component label are discovered even when their names differ.

The policy/binding and webhook transitions are separate API operations.
Admission propagation and any overlap with the old rule-based mutator
must be exercised in the real install/upgrade qualification; the draft
makes no claim of an atomic or measured zero-gap cutover.

## Testing

Completed locally against this draft:

- Unit tests for explicit opt-in, missing image handling, policy/image
  drift, disable cleanup, ownership protection, failed registration
  removal, legacy resource/status cleanup and normal HCO uninstall.
- HCO controller and manifest-tool tests/builds; functional-test compilation.
- An isolated Kubernetes v1.37.0 API server accepted both v1 admission
  resources and exercised server-side dry-run mutation: named compute
  container replacement, preservation of existing fields, missing
  annotations, unrelated Pods, missing VMI owner/compute container,
  idempotence and image updates.
- `make lint`: zero issues. `make go-fix` completed; its helper reports
  workspace exclusions for unrelated nested tooling modules.

The AIE functional test now exercises API-server dry-run mutation and
checks policy creation/removal and absence of the retired registration
and Deployment. It skips bundles without an AIE launcher image.

## Release qualification

[CNV-100753](https://redhat.atlassian.net/browse/CNV-100753) still owns
fresh CNV v5.1 install and v5.0-to-v5.1 upgrade execution, real GPU VM
startup, configured node affinity and native IOMMUFD access. Those checks,
full bundle generation and downstream image delivery are not established
by the local unit or API-server checks.

## References

- [Implementation story](https://redhat.atlassian.net/browse/CNV-98730)
- [Retirement epic](https://redhat.atlassian.net/browse/CNV-98728)
- [Kubernetes MutatingAdmissionPolicy](https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/)
