package ipstacktype

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

func TestIPStackType(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IP Stack Type Suite")
}

var _ = Describe("IP Stack Type", func() {
	DescribeTable("Compute should detect the correct stack type",
		func(entries []openshiftconfigv1.ClusterNetworkEntry, expected string) {
			Expect(Compute(entries)).To(Equal(expected))
		},
		Entry("IPv4 only", []openshiftconfigv1.ClusterNetworkEntry{
			{CIDR: "10.128.0.0/14"},
		}, IPv4SingleStack),
		Entry("IPv6 only", []openshiftconfigv1.ClusterNetworkEntry{
			{CIDR: "fd01::/48"},
		}, IPv6SingleStack),
		Entry("dual stack", []openshiftconfigv1.ClusterNetworkEntry{
			{CIDR: "10.128.0.0/14"},
			{CIDR: "fd01::/48"},
		}, DualStack),
		Entry("empty cluster network", []openshiftconfigv1.ClusterNetworkEntry{}, IPv4SingleStack),
	)

	It("should store and retrieve values", func() {
		Set(DualStack)
		Expect(Get()).To(Equal(DualStack))

		Set(IPv6SingleStack)
		Expect(Get()).To(Equal(IPv6SingleStack))
	})
})
