# Go Code Review Guidelines

Applies to: `**/*.go`

## Code Style

- Use shared constants for feature gate names, annotation keys, and
  label values; flag any duplicated string literals that should be
  constants
- When assigning slices from Kubernetes API objects into other objects,
  use `DeepCopy()` to avoid shared-reference bugs

## Naming

- Acronyms in names must be fully upper-cased (e.g. `VM`, `VMI`, `CDI`, `SSP`)
- Avoid exporting variables, constants, or functions that are not
  needed outside the package
- Catch typos in function/variable names, especially in test helpers
  that may be widely referenced
