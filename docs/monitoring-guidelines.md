## Hyperconverged Cluster Operator Monitoring

### Observability Compatibility Policy

This policy covers all Hyperconverged Cluster Operator observability signals. Like: metrics (and their names, label sets, and types), Prometheus recording rules, and alerting rules. Unless explicitly stated otherwise, these signals are considered implementation details and are subject to change.

- Stability: Hyperconverged Cluster Operator does not guarantee long-term backwards compatibility for observability signals. Names, labels, types, and semantics may change between releases to improve correctness, performance, or operability.
- Deprecation: When feasible, we will deprecate renamed or removed signals by:
  - Marking the old name as Deprecated in documentation.
  - Optionally providing short-lived compatibility recording rules (aliases) that map new signals to old names.
  - Keeping deprecated signals for at least one minor release when possible. In exceptional cases (security, correctness, or scalability), changes may occur without a deprecation window.
- Communication: Material changes will be documented in release notes and reflected in `docs/observability/metrics.md`. Alert and rule updates will also be surfaced via PR descriptions.
- Consumer guidance: Dashboards and alerts should:
  - When creating PromQL queries, expect label sets to change; avoid relying on exhaustive or fixed labels. Select, join, and group by the minimum labels required.
    - Example: Prefer `sum by (namespace)(...)` over `sum by (namespace,pod,container,instance)(...)` when possible.
  - Treat deprecated signals as temporary and migrate to replacements promptly.

Contributors adding or changing observability signals should update documentation, consider temporary compatibility rules if practical, and include migration notes in the PR and release notes.