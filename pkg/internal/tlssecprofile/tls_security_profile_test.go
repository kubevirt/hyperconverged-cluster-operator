package tlssecprofile

import (
	"context"
	"crypto/tls"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
)

var _ = Describe("TLS Security Profile", func() {
	var (
		testScheme *runtime.Scheme

		intermediateTLSSecurityProfile = openshiftconfigv1.TLSSecurityProfile{
			Type:         openshiftconfigv1.TLSProfileIntermediateType,
			Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
		}

		oldTLSSecurityProfile = openshiftconfigv1.TLSSecurityProfile{
			Type: openshiftconfigv1.TLSProfileOldType,
			Old:  &openshiftconfigv1.OldTLSProfile{},
		}
		modernTLSSecurityProfile = openshiftconfigv1.TLSSecurityProfile{
			Type:   openshiftconfigv1.TLSProfileModernType,
			Modern: &openshiftconfigv1.ModernTLSProfile{},
		}
	)

	BeforeEach(func() {
		testScheme = runtime.NewScheme()
		Expect(openshiftconfigv1.Install(testScheme)).To(Succeed())
	})

	DescribeTableSubtree("check TLSSecurityProfile on different configurations ...",
		func(isOnOpenshift bool, clusterTLSSecurityProfile, hcoTLSSecurityProfile, expectedTLSSecurityProfile *openshiftconfigv1.TLSSecurityProfile) {
			var (
				cl client.Client
			)

			BeforeEach(func() {
				setAPIServerProfile(nil)

				clBuilder := fake.NewClientBuilder().WithScheme(testScheme)
				if isOnOpenshift {
					clBuilder.WithRuntimeObjects(&openshiftconfigv1.APIServer{
						ObjectMeta: metav1.ObjectMeta{
							Name: APIServerCRName,
						},
						Spec: openshiftconfigv1.APIServerSpec{
							TLSSecurityProfile: clusterTLSSecurityProfile,
						},
					})
				}

				cl = clBuilder.Build()
			})

			It("check TLSSecurityProfile", func(ctx context.Context) {
				ctx = logr.NewContext(ctx, GinkgoLogr)

				if isOnOpenshift {
					modified, err := Refresh(ctx, cl)
					Expect(err).ToNot(HaveOccurred())
					Expect(modified).To(BeTrue())
				}

				Expect(GetTLSSecurityProfile(hcoTLSSecurityProfile)).To(Equal(expectedTLSSecurityProfile), "should return the expected TLSSecurityProfile")
			})
		},
		Entry(
			"on Openshift, TLSSecurityProfile unset on HCO, should return cluster wide TLSSecurityProfile",
			true,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
		),
		Entry(
			"on Openshift with wrong values, TLSSecurityProfile unset on HCO, should return sanitized cluster wide TLSSecurityProfile - 1",
			true,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileCustomType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].Ciphers,
						MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					},
				},
			},
		),
		Entry(
			"on Openshift with wrong values, TLSSecurityProfile unset on HCO, should return sanitized cluster wide TLSSecurityProfile - 2",
			true,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
			},
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].Ciphers,
						MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					},
				},
			},
		),
		Entry(
			"on Openshift with wrong values, TLSSecurityProfile unset on HCO, should return sanitized cluster wide TLSSecurityProfile - 3",
			true,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers: []string{
							"wrongname1",
							"TLS_AES_128_GCM_SHA256",
							"TLS_AES_256_GCM_SHA384",
							"TLS_CHACHA20_POLY1305_SHA256",
							"ECDHE-ECDSA-AES128-GCM-SHA256",
							"ECDHE-RSA-AES128-GCM-SHA256",
							"ECDHE-ECDSA-AES256-GCM-SHA384",
							"ECDHE-RSA-AES256-GCM-SHA384",
							"ECDHE-ECDSA-CHACHA20-POLY1305",
							"ECDHE-RSA-CHACHA20-POLY1305",
							"wrongname2",
							"DHE-RSA-AES128-GCM-SHA256",
							"DHE-RSA-AES256-GCM-SHA384",
							"wrongname3",
						},
						MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					},
				},
			},
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers: []string{
							"TLS_AES_128_GCM_SHA256",
							"TLS_AES_256_GCM_SHA384",
							"TLS_CHACHA20_POLY1305_SHA256",
							"ECDHE-ECDSA-AES128-GCM-SHA256",
							"ECDHE-RSA-AES128-GCM-SHA256",
							"ECDHE-ECDSA-AES256-GCM-SHA384",
							"ECDHE-RSA-AES256-GCM-SHA384",
							"ECDHE-ECDSA-CHACHA20-POLY1305",
							"ECDHE-RSA-CHACHA20-POLY1305",
							"DHE-RSA-AES128-GCM-SHA256",
							"DHE-RSA-AES256-GCM-SHA384",
						},
						MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					},
				},
			},
		),
		Entry(
			"on Openshift, TLSSecurityProfile set on HCO, should return HCO specific TLSSecurityProfile",
			true,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileOldType,
				Old:  &openshiftconfigv1.OldTLSProfile{},
			},
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileOldType,
				Old:  &openshiftconfigv1.OldTLSProfile{},
			},
		),
		Entry(
			"on k8s, TLSSecurityProfile unset on HCO, should return a default value (Intermediate TLSSecurityProfile)",
			false,
			nil,
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:         openshiftconfigv1.TLSProfileIntermediateType,
				Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
			},
		),
		Entry(
			"on k8s, TLSSecurityProfile unset on HCO, should return HCO specific TLSSecurityProfile)",
			false,
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
			&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			},
		),
		Entry(
			"on k8s, a wrong TLSSecurityProfile is set on HCO, should return sanitized TLSSecurityProfile",
			false,
			nil,
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
			},
			&openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].Ciphers,
						MinTLSVersion: openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					},
				},
			},
		),
	)

	Describe("GetCipherSuitesAndMinTLSVersion", func() {

		BeforeEach(func() {
			setAPIServerProfile(nil)
		})

		It("should return Intermediate ciphers and minTLSVersion when using default profile", func() {
			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(nil)

			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		It("should return Modern ciphers and minTLSVersion when using Modern profile", func() {
			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		It("should return Old ciphers and minTLSVersion when using Old profile", func() {
			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileOldType,
				Old:  &openshiftconfigv1.OldTLSProfile{},
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		It("should return custom ciphers and minTLSVersion when using Custom profile", func() {
			customCiphers := []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"ECDHE-ECDSA-AES128-GCM-SHA256",
			}
			customMinTLSVersion := openshiftconfigv1.VersionTLS12

			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       customCiphers,
						MinTLSVersion: customMinTLSVersion,
					},
				},
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			Expect(ciphers).To(Equal(customCiphers))
			Expect(minTLSVersion).To(Equal(customMinTLSVersion))
		})

		It("should return custom ciphers with TLS 1.3 minTLSVersion", func() {
			customCiphers := []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
			}
			customMinTLSVersion := openshiftconfigv1.VersionTLS13

			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileCustomType,
				Custom: &openshiftconfigv1.CustomTLSProfile{
					TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
						Ciphers:       customCiphers,
						MinTLSVersion: customMinTLSVersion,
					},
				},
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			Expect(ciphers).To(Equal(customCiphers))
			Expect(minTLSVersion).To(Equal(openshiftconfigv1.VersionTLS13))
		})

		It("should return APIServer ciphers when HCO profile is nil", func() {
			setAPIServerProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			})

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(nil)

			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		It("should return HCO ciphers even when APIServer has different profile", func() {
			setAPIServerProfile(&openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileModernType,
				Modern: &openshiftconfigv1.ModernTLSProfile{},
			})

			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type: openshiftconfigv1.TLSProfileOldType,
				Old:  &openshiftconfigv1.OldTLSProfile{},
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		It("should return Intermediate ciphers when profile Type is Custom but Custom field is nil", func() {
			hcoTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
				Type:   openshiftconfigv1.TLSProfileCustomType,
				Custom: nil,
			}

			ciphers, minTLSVersion := GetCipherSuitesAndMinTLSVersion(hcoTLSSecurityProfile)

			// When Custom is nil, SetHyperConvergedProfile sanitizes it with Intermediate defaults
			expectedProfile := openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType]
			Expect(ciphers).To(Equal(expectedProfile.Ciphers))
			Expect(minTLSVersion).To(Equal(expectedProfile.MinTLSVersion))
		})

		Context("selectCipherSuitesAndMinTLSVersion", func() {

			var (
				apiServer *openshiftconfigv1.APIServer
				cl        *commontestutils.HcoTestClient
			)

			BeforeEach(func() {
				apiServer = &openshiftconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: openshiftconfigv1.APIServerSpec{},
				}

				cl = commontestutils.InitClient([]client.Object{apiServer})
			})

			// This test is taken from the old implementation in the validating webhook
			DescribeTable("should consume ApiServer config if HCO one is not explicitly set",
				func(ctx context.Context, initApiTlsSecurityProfile, initHCOTlsSecurityProfile, midApiTlsSecurityProfile, midHCOTlsSecurityProfile, finApiTlsSecurityProfile, finHCOTlsSecurityProfile *openshiftconfigv1.TLSSecurityProfile, initExpected, midExpected, finExpected openshiftconfigv1.TLSProtocolVersion) {
					apiServer.Spec.TLSSecurityProfile = initApiTlsSecurityProfile
					Expect(cl.Update(ctx, apiServer)).To(Succeed())
					_, err := Refresh(ctx, cl)
					Expect(err).ToNot(HaveOccurred())

					_, minTypedTLSVersion := GetCipherSuitesAndMinTLSVersion(initHCOTlsSecurityProfile)
					Expect(minTypedTLSVersion).To(Equal(initExpected))

					apiServer.Spec.TLSSecurityProfile = midApiTlsSecurityProfile
					Expect(cl.Update(ctx, apiServer)).To(Succeed())

					_, err = Refresh(ctx, cl)
					Expect(err).ToNot(HaveOccurred())

					_, minTypedTLSVersion = GetCipherSuitesAndMinTLSVersion(midHCOTlsSecurityProfile)
					Expect(minTypedTLSVersion).To(Equal(midExpected))

					apiServer.Spec.TLSSecurityProfile = finApiTlsSecurityProfile
					Expect(cl.Update(ctx, apiServer)).To(Succeed())

					_, err = Refresh(ctx, cl)
					Expect(err).ToNot(HaveOccurred())

					_, minTypedTLSVersion = GetCipherSuitesAndMinTLSVersion(finHCOTlsSecurityProfile)
					Expect(minTypedTLSVersion).To(Equal(finExpected))
				},
				Entry("nil on APIServer, nil on HCO -> old on API server -> nil on API server",
					nil,
					nil,
					&oldTLSSecurityProfile,
					nil,
					nil,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
				Entry("nil on APIServer, nil on HCO -> modern on HCO -> nil on HCO",
					nil,
					nil,
					nil,
					&modernTLSSecurityProfile,
					nil,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
				Entry("old on APIServer, nil on HCO -> intermediate on HCO -> old on API server",
					&oldTLSSecurityProfile,
					nil,
					&oldTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					&oldTLSSecurityProfile,
					nil,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileOldType].MinTLSVersion,
				),
				Entry("old on APIServer, modern on HCO -> intermediate on HCO -> modern on API server, intermediate on HCO",
					&oldTLSSecurityProfile,
					&modernTLSSecurityProfile,
					&oldTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					&modernTLSSecurityProfile,
					&intermediateTLSSecurityProfile,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileModernType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
					openshiftconfigv1.TLSProfiles[openshiftconfigv1.TLSProfileIntermediateType].MinTLSVersion,
				),
			)

		})

	})

	Describe("MutateTLSConfig", func() {
		It("should use HCO profile if provided", func() {
			SetHyperConvergedTLSSecurityProfile(&oldTLSSecurityProfile)
			setAPIServerProfile(&modernTLSSecurityProfile)

			tlsConfig := &tls.Config{}

			MutateTLSConfig(tlsConfig)

			config, err := tlsConfig.GetConfigForClient(nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.MinVersion).To(Equal(uint16(tls.VersionTLS10)))
			expectedTLSCiphers := []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			}
			Expect(config.CipherSuites).To(Equal(expectedTLSCiphers))
		})

		It("should use APIServer profile if HCO one is not provided", func() {
			SetHyperConvergedTLSSecurityProfile(nil)
			setAPIServerProfile(&modernTLSSecurityProfile)

			tlsConfig := &tls.Config{}

			MutateTLSConfig(tlsConfig)

			config, err := tlsConfig.GetConfigForClient(nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
			expectedTLSCiphers := []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			}
			Expect(config.CipherSuites).To(Equal(expectedTLSCiphers))
		})

		It("should use intermediate profile if both HCO and APIServer profiles are not provided", func() {
			SetHyperConvergedTLSSecurityProfile(nil)
			setAPIServerProfile(nil)

			tlsConfig := &tls.Config{}

			MutateTLSConfig(tlsConfig)

			config, err := tlsConfig.GetConfigForClient(nil)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			expectedTLSCiphers := []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			}
			Expect(config.CipherSuites).To(Equal(expectedTLSCiphers))
		})
	})
})
