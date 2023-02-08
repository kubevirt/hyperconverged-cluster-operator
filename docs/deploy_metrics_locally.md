# Consume metrics locally
This document aims to explain how to deploy and consume all metrics exposed by the Hyperconverged Cluster
Operator (HCO) locally, i.e. using kubevirtci.
The exposed metrics are documented [here](metrics.md).

Metrics are supported in CRC out-of-the-box, but at least 20GB of free memory is recommended.
To deploy metrics, you can target the following command:
```bash
$ crc config set enable-cluster-monitoring true
```
## Deploying kube-prometheus

In order to deploy `kube-prometheus`, it is required to pick a compatible version of `kube-prometheus`
with your local kubernetes cluster version. Refer to the [compatibility section](https://github.com/prometheus-operator/kube-prometheus#compatibility) 
of `kube-prometheus`.

> *Note:* You can set a specific kubernetes cluster version by setting the environmental variable `KUBEVIRT_PROVIDER`.

The next step is to deploy `kube-prometheus` in your local cluster. You can follow the [Quickstart section](https://github.com/prometheus-operator/kube-prometheus#quickstart) of 
`kube-prometheus` to achieve it.



## Deploying HCO Service Monitor

Service monitors tells Prometheus where to scrape the metrics. In this case, we want to get HCO metrics.
For this purpose, the following configuration can be applied:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    app: kubevirt-hyperconverged
    app.kubernetes.io/component: monitoring
    app.kubernetes.io/managed-by: hco-operator
    app.kubernetes.io/part-of: hyperconverged-cluster
  name: kubevirt-hyperconverged-operator-metrics
  namespace: kubevirt-hyperconverged
spec:
  endpoints:
  - bearerTokenSecret:
      key: ""
    port: http-metrics
  namespaceSelector: {}
  selector:
    matchLabels:
      app: kubevirt-hyperconverged
      app.kubernetes.io/component: monitoring
      app.kubernetes.io/managed-by: hco-operator
      app.kubernetes.io/part-of: hyperconverged-cluster
```
## Creating a Service Pointing to HCO Operator

The `ServiceMonitor` needs to point to an existing `Service` in order to fetch the HCO Operator metrics.
In that sense, you can create the service using the following definition:

```yaml
apiVersion: v1
kind: Service
metadata:
  labels:
    app: kubevirt-hyperconverged
    app.kubernetes.io/component: monitoring
    app.kubernetes.io/managed-by: hco-operator
    app.kubernetes.io/part-of: hyperconverged-cluster
  name: kubevirt-hyperconverged-operator-metrics
  namespace: kubevirt-hyperconverged
spec:
  internalTrafficPolicy: Cluster
  ports:
  - name: http-metrics
    port: 8383
    protocol: TCP
    targetPort: 8383
  selector:
    name: hyperconverged-cluster-operator
  sessionAffinity: None
  type: ClusterIP
```


## Port Forwarding Prometheus

Finally, in order to inspect the exposed metrics, we need forward the port `9090`, which corresponds with the Prometheus
deploy. This can be done with the following command:

```bash
$ kubectl -n monitoring port-forward svc/prometheus-k8s 9090
```

Now you can access to the Prometheus dashboard using [http://localhost:9090](http://localhost:9090).