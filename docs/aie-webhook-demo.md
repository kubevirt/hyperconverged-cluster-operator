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
  (handled automatically by the manifest pipeline)

## 1 - Enable the AIE webhook

Set the `hco.kubevirt.io/deployAIEWebhook` annotation:

```bash
kubectl annotate hco kubevirt-hyperconverged -n kubevirt-hyperconverged \
  hco.kubevirt.io/deployAIEWebhook=true
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
OR'd within a rule. Rules can also include an optional `nodeSelector` to inject
node affinity so that matched pods are scheduled onto specific nodes.

```bash
kubectl edit cm kubevirt-aie-launcher-config -n kubevirt-hyperconverged
```

Set the `config.yaml` key to contain your rules:

```yaml
rules:
- name: "nvidia-gpu"
  image: "quay.io/kubevirt/kubevirt-aie/virt-launcher:2603091016-b9f58a81e3-pr10"
  selector:
    deviceNames:
    - "nvidia.com/A100"
    - "nvidia.com/H100"
- name: "labeled-vms"
  image: "quay.io/kubevirt/kubevirt-aie/virt-launcher:2603091016-b9f58a81e3-pr10"
  selector:
    vmLabels:
      matchLabels:
        gpu-workload: "true"
```

HCO will not overwrite your edits to the ConfigMap data during reconciliation.

## 4 - Inspect the ConfigMap

```bash
kubectl get cm kubevirt-aie-launcher-config -n kubevirt-hyperconverged -o jsonpath='{.data.config\.yaml}'
```

## 5 - Disable the webhook

Removing the annotation removes all operand resources:

```bash
kubectl annotate hco kubevirt-hyperconverged -n kubevirt-hyperconverged \
  hco.kubevirt.io/deployAIEWebhook-
```

Verify cleanup:

```bash
kubectl get deploy,mutatingwebhookconfiguration -l app.kubernetes.io/component=aie-webhook -A
# No resources found
```
