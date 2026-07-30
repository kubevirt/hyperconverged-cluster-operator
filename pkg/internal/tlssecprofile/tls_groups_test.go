package tlssecprofile

import (
	"crypto/tls"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
)

var _ = Describe("TLS Groups", func() {
	Describe("CurveIDForTLSGroup", func() {
		DescribeTable("should return the correct CurveID for recognized groups",
			func(group openshiftconfigv1.TLSGroup, expected tls.CurveID) {
				id, ok := CurveIDForTLSGroup(group)
				Expect(ok).To(BeTrue())
				Expect(id).To(Equal(expected))
			},
			Entry("X25519", openshiftconfigv1.TLSGroupX25519, tls.X25519),
			Entry("secp256r1", openshiftconfigv1.TLSGroupSecP256r1, tls.CurveP256),
			Entry("secp384r1", openshiftconfigv1.TLSGroupSecP384r1, tls.CurveP384),
			Entry("secp521r1", openshiftconfigv1.TLSGroupSecP521r1, tls.CurveP521),
			Entry("X25519MLKEM768", openshiftconfigv1.TLSGroupX25519MLKEM768, tls.X25519MLKEM768),
			Entry("SecP256r1MLKEM768", openshiftconfigv1.TLSGroupSecP256r1MLKEM768, tls.SecP256r1MLKEM768),
			Entry("SecP384r1MLKEM1024", openshiftconfigv1.TLSGroupSecP384r1MLKEM1024, tls.SecP384r1MLKEM1024),
		)

		It("should return false for an unknown group", func() {
			id, ok := CurveIDForTLSGroup("UnknownGroup")
			Expect(ok).To(BeFalse())
			Expect(id).To(Equal(tls.CurveID(0)))
		})
	})

	Describe("CurveIDsForTLSGroups", func() {
		It("should convert all recognized groups", func() {
			groups := []openshiftconfigv1.TLSGroup{
				openshiftconfigv1.TLSGroupX25519,
				openshiftconfigv1.TLSGroupSecP256r1,
			}
			curveIDs, unsupported := CurveIDsForTLSGroups(groups)
			Expect(curveIDs).To(Equal([]tls.CurveID{tls.X25519, tls.CurveP256}))
			Expect(unsupported).To(BeEmpty())
		})

		It("should separate unsupported groups", func() {
			groups := []openshiftconfigv1.TLSGroup{
				openshiftconfigv1.TLSGroupX25519,
				"BadGroup",
				openshiftconfigv1.TLSGroupSecP384r1,
				"AnotherBad",
			}
			curveIDs, unsupported := CurveIDsForTLSGroups(groups)
			Expect(curveIDs).To(Equal([]tls.CurveID{tls.X25519, tls.CurveP384}))
			Expect(unsupported).To(Equal([]openshiftconfigv1.TLSGroup{"BadGroup", "AnotherBad"}))
		})

		It("should return nil for nil input", func() {
			curveIDs, unsupported := CurveIDsForTLSGroups(nil)
			Expect(curveIDs).To(BeNil())
			Expect(unsupported).To(BeNil())
		})

		It("should return nil for empty input", func() {
			curveIDs, unsupported := CurveIDsForTLSGroups([]openshiftconfigv1.TLSGroup{})
			Expect(curveIDs).To(BeNil())
			Expect(unsupported).To(BeNil())
		})

		It("should return all unsupported when none are recognized", func() {
			groups := []openshiftconfigv1.TLSGroup{"Foo", "Bar"}
			curveIDs, unsupported := CurveIDsForTLSGroups(groups)
			Expect(curveIDs).To(BeNil())
			Expect(unsupported).To(Equal([]openshiftconfigv1.TLSGroup{"Foo", "Bar"}))
		})
	})

	Describe("ValidTLSGroups", func() {
		It("should return all mapped group names in sorted order", func() {
			names := ValidTLSGroups()
			Expect(names).To(Equal([]string{
				"SecP256r1MLKEM768",
				"SecP384r1MLKEM1024",
				"X25519",
				"X25519MLKEM768",
				"secp256r1",
				"secp384r1",
				"secp521r1",
			}))
		})
	})
})
