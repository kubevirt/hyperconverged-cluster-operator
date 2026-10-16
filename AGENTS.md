# AGENTS.md

Guidance derived from CodeRabbit review history for this directory.

## Review Guidelines

Path-specific review guidelines are in `docs/review_guidelines/`:

- `go.md` — Go code style and testing (`**/*.go`)
- `controllers.md` — Controller/handler logic (`**/*controller*/**/*.go`)
- `monitoring.md` — Metrics and alerts (`pkg/monitoring/**`)
- `unit-tests.md` — Unit test quality (`**/*_test.go`)
- `func-tests.md` — Functional test guidelines (`tests/**/*.go`)
- `docs.md` — Documentation review (`docs/**`)
- `dockerfiles.md` — Dockerfile best practices (`**/Dockerfile*`)
- `deploy.md` — Deployment manifests (`{deploy,manifests}/**`)
- `api.md` — API types and CRD schemas (`api/**`)

## Testing
- In controllers/hyperconverged/hyperconverged_controller_test.go, keep expected KubeVirt feature gate literals independent without exporting or referencing unexported constants from controllers/handlers.
- In controllers/hyperconverged/upgrade_test.go, assert against observable reconciliation behavior and object preservation rather than internal call order or requeue returns.

## API
- When adding or changing default values for any type shared between v1 and v1beta1, update the `+kubebuilder:default` annotation blobs in both `api/v1/hyperconverged_types.go` and `api/v1beta1/hyperconverged_types.go`. The struct definition is shared but the annotation JSON strings are independent and can silently diverge (e.g. `LiveMigrationConfigurations` is defined in v1 but referenced from v1beta1).

## Documentation
- Do not edit docs/api.md or docs/api-v1beta1.md directly; update the underlying Go doc comments on API structs and run make generate-doc.
- In user documentation (e.g. docs/cluster-configuration.md), focus on user-facing behavior rather than internal implementation details.
- Refer to the KubeVirt patch annotation as `kubevirt.kubevirt.io/jsonpatch` in user-facing docs.
- Do not document external API enumerations (e.g. KubeVirt RebootPolicy values) or rigid performance recommendations in user docs; these are subject to change outside this repo.

## Webhooks
- In pkg/webhooks/validator/validator.go, only copy required.Spec when simulating dry-run updates (patches must not touch metadata), and rely on the controller-runtime cached client without adding extra request timeouts.

## Node Architecture
- In pkg/internal/nodeinfo/architectures.go, preserve the single control-plane architecture invariant by guarding default selection with len(cpArchesList) > 0.

## Build & CI
- In Makefile, do not make build-crd-creator depend on generate, as image builds cannot run generation steps; declare generation targets explicitly only where needed.
- In hack/run-tests-in-container.sh and other set -e shell scripts, guard best-effort cleanup and diagnostic logging against non-zero exit codes to prevent aborting error handling.
- In GitHub Actions workflows, pin actions to full commit SHAs and include the human-readable version in a comment. Use mutable tags only when an explicit repository or organization policy documents the review and update controls.

## RBAC
- In controllers/handlers/observabilitycontroller/rbac.go, preserve Service and Secret RBAC permissions required by the observability controller metrics operand.

## Upgrades
- During upgrades, retain explicitly configured legacy values (such as parallelOutboundMigrationsPerNode: 2); apply new default values only when fields are unset.

