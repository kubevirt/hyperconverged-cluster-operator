# Tools

## Rotating Certificates

To rotate certificates for all components managed by the HyperConverged Operator, type:

```
export KUBECONFIG=<wherever>
export KUBECTL_BINARY=kubectl
./rotate-certs.sh -n kubevirt-hyperconverged
```

During the rotation, the following things may occur:

 * Migrations will be cancelled
 * Image uploads will be cancelled
 * VNC and Console connections will be closed

After the rotation is done, all operations will continue as usual.
VirtualMachine and VirtualMachineInstance workloads will not be affected.
