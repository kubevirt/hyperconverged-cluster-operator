# Intro
The KubeVirt Hyperconverged Cluster Operator (HCO) is delivered and deployed on a running OCP/OKD cluster using the kustomize method. 

# Deploy Modes
HCO can be delivered and deployed in several configurations, corresponding to several pre-built kustomize overlays, which are explained below.
## Delivery
### Marketplace
This method is taking advantage of CatalogSourceConfig, pointing to an OperatorSource, which makes the operator available on OLM OperatorHub.
To use this method, set environment variable "MARKETPLACE_MODE" to "true".
### Image Registry
This method is delivering the operator's bundle image via a grpc protocol from an image registry.
To use this method, set environment variable "MARKETPLACE_MODE" to "false".
### Content-Only
To make HCO available for deployment in the cluster, without actually deploy it, set "CONTENT_ONLY" to "true". That will stop script execution before entering the deployment phase.

## Deployment
### Private Repo
If the operator source is located in a private Quay.io registry, you should set "PRIVATE_REPO" to "true" and provide credentials using "QUAY_USERNAME" and "QUAY_PASSWORD" environment variables.
### KVM Emulation
If KVM emulation is required on your environment, set "KVM_EMULATION" to "true". 

## Customizations
Existing manifests in this repository are representing an HCO deployment with default settings.
In order to make customizations to your deployment, you need to set up other environment variables and create kustomize overlays to override default settings.
### Change Deployment Namespace
The default namespace is `kubevirt-hyperconverged`.
In order to change that to a custom value, you should edit `namespace.yaml` and update its `metadata.name` value.

### Modify HCO Channel and Version
Create a Subscription patch to reflect the desired version and channel.
```
cat > subscription.patch.yaml << EOF
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: hco-operatorhub
spec:
  startingCSV: kubevirt-hyperconverged-operator.v${HCO_VERSION}
  channel: "${HCO_CHANNEL}"
```

# Deploy
When customizations are ready, run `./deploy_kustomize.sh`.
The script will prepare and submit kustimized manifests directories to the cluster.