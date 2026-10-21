# Unit Test Review Guidelines

Applies to: `**/*_test.go`

- Focus on test coverage and test quality
- Review Ginkgo/Gomega usage patterns
- Check for proper test setup and teardown
- Verify test scenarios cover error cases and edge conditions
- Ensure tests are deterministic and not flaky
- Do not add tests whose sole purpose is verifying logger output
- When enabling a KubeVirt feature gate by default, update the
  `expectedFeatureGates` slice in the "should create all managed resources"
  test in `controllers/hyperconverged/hyperconverged_controller_test.go`
- Assert against observable behavior (resulting state, return values, errors)
  rather than internal implementation details such as call order. only
  assert on incidental implementation artifacts (e.g., a specific requeue
  value) when they are part of the function's documented contract
