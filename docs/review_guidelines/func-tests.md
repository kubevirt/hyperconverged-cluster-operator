# Functional Test Review Guidelines

Applies to: `tests/**/*.go`

- Use Ginkgo Labels to categorize tests (e.g., Label("Hypervisors"))
- Prefer unit tests over functional tests when possible
- Restore modified resources using DeferCleanup to avoid test pollution
- Prefer Patch over Update for API operations in tests
- Prefer helper functions that return errors rather than
  validating with Gomega matchers internally
- In helper functions use `GinkgoHelper()`, not Gomega offset functions
