Place Perses CR manifests here (YAML files) to be applied by the observability controller.

Expected kinds (apiVersion subject to the Observability Operator installed in the cluster):
- PersesDashboard (e.g. apiVersion: observability.rhobs/v1alpha1)
- PersesDatasource (e.g. apiVersion: observability.rhobs/v1alpha1)

Manifests will be server-side applied into the operator namespace at runtime.

Example: memory-load dashboard
- Source: https://github.com/fabiand/perses-dashboards/blob/main/manifests/memory-load.yaml
- Copy the file here as `memory-load.yaml`. The controller will override `metadata.namespace` to the operator namespace.


