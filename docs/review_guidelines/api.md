# API Types Review Guidelines

Applies to: `api/**`

- API types must follow [OpenShift API conventions](https://github.com/openshift/enhancements/blob/master/dev-guide/api-conventions.md)
- The v1beta1 API is frozen — verify that no v1beta1 types are modified;
  any API change must be reflected in `api/v1beta1/conversion_fuzz_test.go`
- New spec fields that participate in conversion must have round-trip conversion tests
- Ensure backward compatibility with previous versions of HCO
- When adding or changing default values for a type shared between v1 and
  v1beta1, update both `+kubebuilder:default` annotation blobs only when the
  change is intended and permitted for both API versions. Preserve the frozen
  v1beta1 default for v1-only changes.
