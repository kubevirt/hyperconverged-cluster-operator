# Contributing to Hyperconverged Cluster Operator

## ***This document is a work in progress***

## Contributing to the HyperConverged API

### Add new Feature Gate

1. Add the new feature gate to the HyperConvergedFeatureGates struct
   in [pkg/apis/hco/v1beta1/hyperconverged_types.go](pkg/apis/hco/v1beta1/hyperconverged_types.go)
   - make sure the name of the feature gate field is as the feature gate field in the target operand, including casing.
     But it also must start with a capital letter.
   - Set the field type to `FeatureGate`.
   - Make sure the json name in the json tag is valid (e.g. starts with a small cap).
   - add open-api annotations:
      - add detailed description in the comment
      - default annotation
      - optional annotation

for example:

  ```golang
  // Allow migrating a virtual machine with SRIOV interfaces.
  // When enabled virt-launcher pods of virtual machines with SRIOV
  // interfaces run with CAP_SYS_RESOURCE capability.
  // This may degrade virt-launcher security.
  // +optional
  // +kubebuilder:default=false
  SRIOVLiveMigration FeatureGate `json:"sriovLiveMigration,omitempty"`
  ```

1. Add the new flag to the default of the FeatureGates field in the `HyperConvergedSpec` struct
1. Add the new flag to the `"Should return true for each enabled gate"`,
   in [pkg/apis/hco/v1beta1/hyperconverged_types_test.go](pkg/apis/hco/v1beta1/hyperconverged_types_test.go)
1. Add the new flag to the relevant operator handler. Currently this is only supported for KubeVirt. For KubeVirt, do
   the following:
   In [pkg/controller/operands/kubevirt.go](pkg/controller/operands/kubevirt.go)
   - Add a constant for the flag name in the constant block marked with the `// KubeVirt FeatureGates` comment.
   - Add this new constant to the `kvFeatureGateList` slice.

   The code that uses this is in `getKvFeatureGateList` function. This function takes only KubeVirt feature gates from
   the currently **enabled** feature gares in the HyperConverged CR. To add a new feature gate in another operand, a
   implement a similar logic in this operand handler file.
