# AIE Webhook Demo

This demo walks through enabling and configuring the AIE (Accelerated
Infrastructure Enablement) webhook via HCO. The webhook intercepts
virt-launcher Pod creation and replaces the compute container image based on
configurable rules, allowing clusters to run specialised launcher builds for
VMs that use specific devices or carry specific labels.

## Prerequisites

* A cluster with HCO deployed (e.g. `make cluster-up && make cluster-sync`)
* `kubectl` configured against the cluster
* The `AIE_WEBHOOK_IMAGE` environment variable set in the operator Deployment
  (handled automatically by `cluster/sync.sh`)

## 1 - Enable the AIE webhook

Flip the `deployAIEWebhook` feature gate:

```bash
kubectl patch hco kubevirt-hyperconverged -n kubevirt-hyperconverged --type=merge -p '
  {"spec":{"featureGates":{"deployAIEWebhook": true}}}'
```

## 2 - Verify the operand resources

HCO creates the full set of webhook resources:

```bash
kubectl get sa,deploy,svc,cm,clusterrole,clusterrolebinding,mutatingwebhookconfiguration \
  -l app.kubernetes.io/component=aie-webhook -A
```

Expected output:

```
NAMESPACE                  NAME                                        ...
kubevirt-hyperconverged    serviceaccount/kubevirt-aie-webhook
kubevirt-hyperconverged    deployment.apps/kubevirt-aie-webhook        1/1
kubevirt-hyperconverged    service/kubevirt-aie-webhook                443/TCP
kubevirt-hyperconverged    configmap/kubevirt-aie-launcher-config
                           clusterrole/kubevirt-aie-webhook
                           clusterrolebinding/kubevirt-aie-webhook
                           mutatingwebhookconfiguration/kubevirt-aie-webhook
```

## 3 - Configure launcher replacement rules

Edit the `kubevirt-aie-launcher-config` ConfigMap directly to add rules. Each
rule specifies a replacement image and a selector that matches on device
resource names (`deviceNames`) and/or VM labels (`vmLabels`). The selectors are
OR'd within a rule.

```bash
kubectl edit cm kubevirt-aie-launcher-config -n kubevirt-hyperconverged
```

Set the `config.yaml` key to contain your rules:

```yaml
rules:
- name: "nvidia-gpu"
  image: "registry.example.com/virt-launcher-gpu:v1.0"
  selector:
    deviceNames:
    - "nvidia.com/A100"
    - "nvidia.com/H100"
- name: "labeled-vms"
  image: "registry.example.com/virt-launcher-custom:v2.0"
  selector:
    vmLabels:
      matchLabels:
        workload-type: "ai-inference"
```

HCO will not overwrite your edits to the ConfigMap data during reconciliation.

## 4 - Inspect the ConfigMap

```bash
kubectl get cm kubevirt-aie-launcher-config -n kubevirt-hyperconverged -o jsonpath='{.data.config\.yaml}'
```

## 5 - Disable the webhook

Setting the feature gate to `false` removes all operand resources:

```bash
kubectl patch hco kubevirt-hyperconverged -n kubevirt-hyperconverged --type=merge -p '
  {"spec":{"featureGates":{"deployAIEWebhook": false}}}'
```

Verify cleanup:

```bash
kubectl get deploy,mutatingwebhookconfiguration -l app.kubernetes.io/component=aie-webhook -A
# No resources found
```
