# HyperConverged API v1

## Abstract

This document proposes the graduation of the `HyperConverged` API from `v1beta1`
to `v1`, marking its transition to a stable, production-ready API version. The
`HyperConverged` API serves as the interface to the Hyperconverged Cluster
Operator (HCO) and acts as the single source for OpenShift Virtualization
configuration.

Since the current `v1beta1` API was introduced a few years ago, we have learned
that several design decisions were not optimal and require improvement. The new
`v1` API is designed to address the following key issues with the `v1beta1` API:

1. The FeatureGates field needs to be redesigned
2. Several fields are related to deprecated features and are not in use
3. The organization of the fields in the API is somewhat chaotic. Reorganizing
   the fields will help users locate the correct configuration settings more
   easily.

The new `v1` API will replace the current `v1beta1` API. This document describes
the changes from v1beta1, the migration process and the implementation details.

## API Changes

### New Feature Gates API
The FeatureGates structure in `v1beta`, is based on named fields, which limits
its utility in the feature graduation process because we cannot remove feature
gates from the API once they have been added. As a result, the process of adding
new feature gates to the API is long and complicated.

The new design aims to simplify the addition and removal of feature gates, and
shall allow any feature gate to pass through the graduation phases:

* `Alpha`: The feature gate is disabled by default. Explicitly adding it will
  enable the feature.
* `Beta`: The feature gate is enabled by default. The user can disable it from
  the API.
* `Generally Available` (`GA`): The feature gate is deprecated and ignored. The
  feature is now a fundamental part of HCO's behavior and cannot be disabled.
* `Dropped`: The feature gate is deprecated and ignored. The feature is no
  longer supported and cannot be activated.

#### FeatureGates API Design
Each feature gate is an object with a `name` field and a `state` field. The
`name` field is the feature gate name. The `state` field values are `"Enabled"`
or `"Disabled"`. The default value for the `state` field is `"Enabled"`. If
the feature is enabled, the state field will not be presented when reading
the `HyperConverged` custom resource.

The `featureGates` field is a list of feature gate objects.

#### Expose the List of Available Feature Gates
The hyperconvergeds CRD will contain the list of all the available feature
gates, their phase (`GA`, `Beta`, Alpha or `Deprecated`) and an optional
description. The CRD text is generated from the golang field comment.
This comment - the comment of the `featureGate` field - will be
auto-generated from a common source of trough, that will also be used for
the feature gate logic.

This will make the list of the available feature gates accessible using `oc
explain` and from the OCP/OKD console (UI).

#### API Example
```yaml
apiVersion: hco.kubevirt.io/v1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
spec:
  ...
  featureGates:
    - name: downwardMetrics # this feature gate is enabled
    - name: deployKubeSecondaryDNS # this feature gate is enabled
      state: Enabled
    - name: videoConfig # this feature gate is disabled
      state: Disabled
```

#### Validation
When creating or updating the HyperConverged CR, HCO will return a warning in
the following cases, but will still respect the user input:

* The feature gate is not known
* The feature gate is deprecated, either if it is GA, or was dropped.
* The feature gate is in alpha phase (disabled by default), and the `state`
  field is `"Enabled"`. The warning message will suggest removing the
  feature gate.
* The feature gate is in beta phase (enabled by default), and the `state` field
  is `"Disabled"`. The warning message will suggest removing the feature gate.

### Field Deprecation
Several fields have become deprecated over the years and will be removed from
the `v1` API:
* `localStorageClassName`
* `vddkInitImage`
* `tektonPipelinesNamespace`
* `tektonTasksNamespace`
* `mediatedDevicesConfiguration.mediatedDevicesTypes`
* `obsoleteCPUs.minCPUModel`

### Hierarchical structure or Grouping Fields by Subjects
The current API structure is relatively flat. Some fields are grouped under
common subject areas, while others are placed directly under the `HyperConverged`
`spec` field. This structure creates a cognitive load when reading or updating
the `HyperConverged` CR, and make it harder to find the right filed to modify
when configuring the KubeVirt system.

* In `v1`, the fields will be grouped by a common topic or logic, to ease the
* maintainability of the `HyperConverged` CR. 
* 
#### Node Placement:
In `v1beta1`, the infra and workloads fields are of type `HyperConvergedConfig`,
which contains only the single `nodePlacement` field.
This structure does not make a lot of sense, and can be improved.

`v1beta1` example:
```yaml
apiVersion: hco.kubevirt.io/v1beta1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
spec:
  infra:
    nodePlacement:
      nodeSelector:
        some-label: some-value
  workloads:
    nodePlacement:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: some-other-label
                operator: In
                values:
                - some-other-value
                - alternative-value
```

Instead, we will introduce a new `nodePlacements` field under `spec`, containing
both `infra` and `workloads` fields, each of type `corev1.nodePlacement`. The
setting above will look like this in `v1`:
```yaml
apiVersion: hco.kubevirt.io/v1beta1
kind: HyperConverged
metadata:
  name: kubevirt-hyperconverged
spec:
  nodePlacements:
    infra:
      nodeSelector:
        some-label: some-value
    workloads:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: some-other-label
                operator: In
                values:
                - some-other-value
                - alternative-value
```

#### Virtualization
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `virtualization`:
* `liveMigrationConfig`
* `permittedHostDevices`
* `mediatedDevicesConfiguration`
* `workloadUpdateStrategy`
* `obsoleteCPUs`
* `evictionStrategy`
* `virtualMachineOptions` (See [below](#VirtualMachineOptions) for changes
   to this field)
* `higherWorkloadDensity`
* `liveUpdateConfiguration`
* `ksmConfiguration`

#### VirtualMachineOptions
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the already existing `virtualMachineOptions` field:
* `defaultCPUModel`
* `defaultRuntimeClass`

#### Storage
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `storage` field:
* `vmStateStorageClass`
* `scratchSpaceStorageClass`
* `storageImport`

#### Security
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `security` field:
* `certConfig`
* `tlsSecurityProfile`

#### Networking
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `networking` field:
* `kubeSecondaryDNSNameServerIP`
* `kubeMacPoolConfiguration`
* `networkBinding`

#### WorkloadSources
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `workloadSources` field:
* `commonTemplatesNamespace`
* `dataImportCronTemplates`
* `commonBootImageNamespace`
* `enableCommonBootImageImport`
* `instancetypeConfig`
* `CommonInstancetypesDeployment`
   **note**: should be renamed to `commonInstancetypesDeployment` (starts
   with a lowercase ‘c’)

#### Deployment
In `v1beta1`, the following fields are directly under `spec`. In `v1`, they
will be moved under the new `deployment` field:
* `deployVmConsoleProxy`

## Development Phases
### v1.18: Introduction of API `v1`
The following is planned for this phase:
* The new API `v1` is introduced in the hyperconvergeds CRD, as a served
  version, but not stored.
* Implementation of a conversion webhook and its deployment (k8s w/ or w/o OLM,
  OCP/OKD with OLM v0 and `v1`).
* Any API change, like adding a new field, must be added to both API versions.
* If new fields are added, make sure they can be set and read from both APIs.
* Add new functional test to make sure the `v1` API is usable, and the
  conversion webhook is working as expected, to allow seamless usage of both
  API versions.

### v1.19: HCO code uses API `v1`
The following is planned for this phase:
* Modify the HCO code, both production and testing, to use API `v1` each time it
  reads, writes or updates the `HyperConverged` CR. That also includes any script
  in the hyperconverged-cluster-operator repository, and the repositories it
  uses for CI.
* Any API change, like adding a new field, must be added to both API versions.
* Add new functional test to make sure the `v1beta1` API is usable, and the
  conversion webhook is working as expected, to allow seamless usage of both
  API versions.

### v1.20: API `v1` becomes the stored version; `v1beta1` is deprecated
The following is planned for this phase:
* The hyperconvergeds CRD defines `v1` as the stored version, and the `v1beta1`
  version as deprecated. The CRD will also define a deprecation message.
* Any API change, like adding a new field, must be added to both API versions.
* OCP/OKD UI: move to API v1

### v1.22: Dropping API `v1beta1`
The following is planned for this phase:
* The CRD will no longer contain the `v1beta1` API.
* Any `v1beta1` usage in testing will be removed.
