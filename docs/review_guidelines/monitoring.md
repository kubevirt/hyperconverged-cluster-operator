# Monitoring, Metrics, and Alerts Review Guidelines

Applies to: `pkg/monitoring/**`

- Keep monitoring rules organized under `pkg/monitoring/`
- New metrics must have corresponding unit tests that verify the
  metric is registered, labeled correctly, and set under the
  expected conditions
- Alert definitions must include: expr, for, severity,
  operator_health_impact, summary, and description annotations
- A corresponding runbook must exist (or have an open PR) in the
  kubevirt/monitoring repository for each alert
