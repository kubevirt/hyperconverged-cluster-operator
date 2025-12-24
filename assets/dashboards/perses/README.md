Perses assets shipped by HCO are embedded into the operator image and reconciled by a dedicated Perses controller.

Layout
assets/dashboards/perses/
├── dashboards/   ← PersesDashboard YAMLs managed by HCO
└── datasources/  ← PersesDatasource YAMLs managed by HCO

Dashboards allowlist
- HCO only reconciles dashboards whose names are in an internal allowlist controlled by HCO.
- Defaults include (shipped by HCO):
  - perses-dashboard-node-memory-overview

Default datasource
- Name: perses-thanos-datasource
- Targets the in-cluster Thanos Querier through an HTTP proxy.

Prerequisites
- Observability Operator installs Perses and CRDs (PersesDashboard, PersesDatasource).
- If CRDs are not present, the controller defers reconciliation until they are available.
- To render dashboards in the Console, ensure the Monitoring UIPlugin (Perses) is enabled.

