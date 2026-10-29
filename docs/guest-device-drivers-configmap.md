# Guest Device Drivers Reference ConfigMap

HCO can deploy a reference ConfigMap, named `kubevirt-guest-device-drivers-config`, that
maps VirtIO guest device IDs to the driver versions shipped by the distributor. The
[observability controller](https://github.com/kubevirt/kubevirt-observability-controller)
reads this ConfigMap to report the `kubevirt_guest_device_driver_latest_version_info`
metric, which allows alerting on VirtualMachines running outdated guest device drivers.
See [VEP #385: Guest Device Info Metrics](https://github.com/kubevirt/enhancements/blob/main/veps/sig-observability/385-guest-device-info/vep.md#driver-version-reference-configmap)
for the full design.

## Build-time content injection

The content of the ConfigMap is **not** part of the HCO sources. Distributors inject it at
**build time** by adding a `drivers.yaml` file under the `guestDeviceDrivers/` directory of
the operator image:

```
guestDeviceDrivers/
└── drivers.yaml
```

The file is read from the operator's working directory when the operand handlers are first
initialized. The expected file format is:

```yaml
version: v1alpha1
devices:
  - deviceID: "1041"
    deviceName: "VirtIO network device"
    current:
      - guestOSVersionID: "2022"
        driverVersion: "100.95.104.26200"
```

Upstream builds do not ship this file.

## Effects

Whenever HCO deploys the observability controller with the `deployObservabilityController`
feature gate enabled, it also deploys the `guest-device-drivers-role` Role and RoleBinding,
regardless of whether the operator image ships `guestDeviceDrivers/drivers.yaml`. The Role
restricts `get`, `list` and `watch` to the single reference ConfigMap through
`resourceNames`, matching the scoping [VEP #385](https://github.com/kubevirt/enhancements/blob/main/veps/sig-observability/385-guest-device-info/vep.md#driver-version-reference-configmap)
requires for the observability controller's watch on this ConfigMap.

The `kubevirt-guest-device-drivers-config` ConfigMap itself is only deployed when the
operator image also ships the reference file; otherwise no ConfigMap is created. Once
deployed, its data is reconciled back to the injected reference content on every
reconciliation, like any other opinionated ConfigMap managed by HCO. Disabling the feature
gate makes HCO remove the ConfigMap it created, but it leaves a ConfigMap of the same name
created by anyone else untouched.

## CI testing

The functional-test image, built from [build/Dockerfile.okd](../build/Dockerfile.okd),
ships a sample reference file: [ci-test-files/drivers.yaml](../ci-test-files/drivers.yaml)
is copied to `guestDeviceDrivers/drivers.yaml`.

The functional test in
[tests/func-tests/observability_controller_test.go](../tests/func-tests/observability_controller_test.go)
checks that the ConfigMap is deployed when the feature gate is enabled. Like the
quickstart, dashboard and image-stream tests, it reads its expectations from the test
configuration file ([hack/testFiles/test_config.yaml](../hack/testFiles/test_config.yaml)):

```yaml
guestDeviceDrivers:
  expectedConfigMap: kubevirt-guest-device-drivers-config
```

When the `guestDeviceDrivers.expectedConfigMap` entry is empty or absent, the test is
skipped — use this when running against an operator image that does not ship the
reference file.
