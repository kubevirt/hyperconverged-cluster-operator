# API Types Review Guidelines

Applies to: `api/**`

- API types must follow [OpenShift API conventions](https://github.com/openshift/enhancements/blob/master/dev-guide/api-conventions.md)
- The v1beta1 API is frozen — verify that no v1beta1 types are modified;
  any API change must be reflected in `api/v1beta1/conversion_fuzz_test.go`
- New spec fields that participate in conversion must have round-trip conversion tests
- Ensure backward compatibility with previous versions of HCO
- When adding or changing default values for any type shared between v1 and
  v1beta1, update the `+kubebuilder:default` annotation blobs in both
  `api/v1/hyperconverged_types.go` and `api/v1beta1/hyperconverged_types.go`.
  The struct definition is shared but the annotation JSON strings are
  independent and can silently diverge (e.g. `LiveMigrationConfigurations`
  is defined in v1 but referenced from v1beta1)
