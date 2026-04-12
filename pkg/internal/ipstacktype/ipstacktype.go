package ipstacktype

import (
	"sync/atomic"

	openshiftconfigv1 "github.com/openshift/api/config/v1"
	"k8s.io/utils/net"
)

const (
	IPv4SingleStack = "IPv4SingleStack"
	IPv6SingleStack = "IPv6SingleStack"
	DualStack       = "DualStack"
)

var value atomic.Value

func Set(v string) {
	value.Store(v)
}

func Get() string {
	if v := value.Load(); v != nil {
		return v.(string)
	}
	return IPv4SingleStack
}

func Compute(clusterNetworks []openshiftconfigv1.ClusterNetworkEntry) string {
	hasIPv4, hasIPv6 := false, false
	for _, n := range clusterNetworks {
		if net.IsIPv4CIDRString(n.CIDR) {
			hasIPv4 = true
		}
		if net.IsIPv6CIDRString(n.CIDR) {
			hasIPv6 = true
		}
	}
	switch {
	case hasIPv4 && hasIPv6:
		return DualStack
	case hasIPv6:
		return IPv6SingleStack
	default:
		return IPv4SingleStack
	}
}
