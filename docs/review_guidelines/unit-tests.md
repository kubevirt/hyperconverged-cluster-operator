# Unit Test Review Guidelines

Applies to: `**/*_test.go`

- Focus on test coverage and test quality
- Review Ginkgo/Gomega usage patterns
- Check for proper test setup and teardown
- Verify test scenarios cover error cases and edge conditions
- Ensure tests are deterministic and not flaky
- Do not add tests whose sole purpose is verifying logger output
- When enabling a feature gate by default, update the test in
  `controllers/hyperconverged/hyperconverged_controller_test.go`. Keep
  expected KubeVirt feature gate literals independent without exporting or
  referencing unexported constants from `controllers/handlers`
- Assert against observable reconciliation behavior and object preservation
  rather than internal call order or requeue returns
