# API Types Review Guidelines

Applies to: `api/**`

- API types must follow [OpenShift API conventions](https://github.com/openshift/enhancements/blob/master/dev-guide/api-conventions.md)
- The v1beta1 API is frozen — verify that no v1beta1 types are modified;
  any API change must be reflected in `api/v1beta1/conversion_fuzz_test.go`
- New spec fields that participate in conversion must have round-trip conversion tests
- Ensure backward compatibility with previous versions of HCO
