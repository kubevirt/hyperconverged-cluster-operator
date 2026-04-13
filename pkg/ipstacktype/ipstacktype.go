package ipstacktype

import internal "github.com/kubevirt/hyperconverged-cluster-operator/pkg/internal/ipstacktype"

const (
	IPv4SingleStack = internal.IPv4SingleStack
	IPv6SingleStack = internal.IPv6SingleStack
	DualStack       = internal.DualStack
)

var (
	Set     = internal.Set
	Get     = internal.Get
	Compute = internal.Compute
)
