# Go Code Review Guidelines

Applies to: `**/*.go`

## Code Style

- Use shared constants for feature gate names, annotation keys, and
  label values; flag any duplicated string literals that should be
  constants
- When assigning slices from Kubernetes API objects into other objects,
  use `DeepCopy()` to avoid shared-reference bugs

## Imports
The imports must be added in up to four groups. Each group must be sorted.
- The first import group is golang standard library
- The second group are third-party packages
- The third group is packages from the kubevirt organization; i.e packages that belong to `kubevirt.io` or
  `github.com/kubevirt`
- The fourth and last group is any group from this repository.

## Naming

- Acronyms in names must be fully upper-cased (e.g. `VM`, `VMI`, `CDI`, `SSP`)
- Avoid exporting variables, constants, or functions that are not
  needed outside the package
- Catch typos in function/variable names, especially in test helpers
  that may be widely referenced

## Value Assignments
- when assigning pointer values, use the builtin `new` function, instead of imported 
  functions like `ptr.To()`

# Modernizing
To make sure your code uses the latest golang features, and is up-to-date, run
```shell
make go-fix
```

## Validation
Run the linters and fix any warning, to make sure the changes match the project standards.
To run the linters, use
```shell
make lint
```
