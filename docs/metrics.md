# Hyperconverged Cluster Operator metrics

| Name | Kind | Type | Description |
|------|------|------|-------------|
| kubevirt_hco_dataimportcrontemplate_with_architecture_annotation | Metric | Gauge | Indicates whether the DataImportCronTemplate has the ssp.kubevirt.io/dict.architectures annotation (1) or not (0) |
| kubevirt_hco_dataimportcrontemplate_with_supported_architectures | Metric | Gauge | Indicates whether the DataImportCronTemplate has supported architectures (1) or not (0) |
| kubevirt_hco_hyperconverged_cr_exists | Metric | Gauge | Indicates whether the HyperConverged custom resource exists (1) or not (0) |
| kubevirt_hco_memory_overcommit_percentage | Metric | Gauge | Indicates the cluster-wide configured VM memory overcommit percentage |
| kubevirt_hco_misconfigured_descheduler | Metric | Gauge | Indicates whether the optional descheduler is not properly configured (1) to work with KubeVirt or not (0) |
| kubevirt_hco_out_of_band_modifications_total | Metric | Counter | Count of out-of-band modifications overwritten by HCO |
| kubevirt_hco_single_stack_ipv6 | Metric | Gauge | Indicates whether the underlying cluster is single stack IPv6 (1) or not (0) |
| kubevirt_hco_system_health_status | Metric | Gauge | Indicates whether the system health status is healthy (0), warning (1), or error (2), by aggregating the conditions of HCO and its secondary resources |
| kubevirt_hco_unsafe_modifications | Metric | Gauge | Count of unsafe modifications in the HyperConverged annotations |
| cluster:kubevirt_hco_operator_health_status:count | Recording rule | Gauge | Indicates whether HCO and its secondary resources health status is healthy (0), warning (1) or critical (2), based both on the firing alerts that impact the operator health, and on kubevirt_hco_system_health_status metric |
| cluster:vmi_request_cpu_cores:sum | Recording rule | Gauge | Sum of CPU core requests for all running virt-launcher VMIs across the entire KubeVirt cluster |
| cnv_abnormal | Recording rule | Gauge | Monitors resources for potential problems |
| kubevirt_hyperconverged_operator_health_status | Recording rule | Gauge | [Deprecated] Indicates whether HCO and its secondary resources health status is healthy (0), warning (1) or critical (2), based both on the firing alerts that impact the operator health, and on kubevirt_hco_system_health_status metric |

## Developing new metrics

All metrics documented here are auto-generated and reflect exactly what is being
exposed. After developing new metrics or changing old ones please regenerate
this document.
