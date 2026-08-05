package handlers

import (
	"context"
	"maps"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	openshiftconfigv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	vmfr "kubevirt.io/vm-file-restore-operator/api/v1alpha1"

	hcov1 "github.com/kubevirt/hyperconverged-cluster-operator/api/v1"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/common"
	"github.com/kubevirt/hyperconverged-cluster-operator/controllers/commontestutils"
	"github.com/kubevirt/hyperconverged-cluster-operator/pkg/tlssecprofile"
	hcoutil "github.com/kubevirt/hyperconverged-cluster-operator/pkg/util"
)

var _ = Describe("VMFileRestore tests", func() {
	var (
		hco *hcov1.HyperConverged
		req *common.HcoRequest
		cl  client.Client
	)

	BeforeEach(func() {
		hco = commontestutils.NewHco()
		req = commontestutils.NewReq(hco)
	})

	Context("test NewFileRestoreOperator", func() {
		oldTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type: openshiftconfigv1.TLSProfileOldType,
			Old:  &openshiftconfigv1.OldTLSProfile{},
		}
		intermediateTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type:         openshiftconfigv1.TLSProfileIntermediateType,
			Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
		}
		modernTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type:   openshiftconfigv1.TLSProfileModernType,
			Modern: &openshiftconfigv1.ModernTLSProfile{},
		}
		customTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type: openshiftconfigv1.TLSProfileCustomType,
			Custom: &openshiftconfigv1.CustomTLSProfile{
				TLSProfileSpec: openshiftconfigv1.TLSProfileSpec{
					Ciphers:       []string{"ECDHE-RSA-AES128-GCM-SHA256"},
					MinTLSVersion: openshiftconfigv1.VersionTLS12,
				},
			},
		}

		It("should have all default fields", func() {
			origFunc := tlssecprofile.GetTLSSecurityProfile
			tlssecprofile.GetTLSSecurityProfile = func(_ *openshiftconfigv1.TLSSecurityProfile) *openshiftconfigv1.TLSSecurityProfile {
				return intermediateTLSSecurityProfile
			}
			DeferCleanup(func() {
				tlssecprofile.GetTLSSecurityProfile = origFunc
			})

			cr := NewFileRestoreOperator(hco)

			Expect(cr.Name).To(Equal("vm-file-restore-" + hco.Name))
			Expect(cr.Namespace).To(Equal(hco.Namespace))
			Expect(cr.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
			Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(intermediateTLSSecurityProfile)))
		})

		Context("TLSSecurityProfile", func() {
			It("should set Old TLS profile from HCO", func() {
				hco.Spec.Security.TLSSecurityProfile = oldTLSSecurityProfile

				cr := NewFileRestoreOperator(hco)

				Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(oldTLSSecurityProfile)))
				Expect(cr.Spec.TLSSecurityProfile.Type).To(Equal(vmfr.TLSProfileType(openshiftconfigv1.TLSProfileOldType)))
				Expect(cr.Spec.TLSSecurityProfile.Old).ToNot(BeNil())
				Expect(cr.Spec.TLSSecurityProfile.Intermediate).To(BeNil())
				Expect(cr.Spec.TLSSecurityProfile.Custom).To(BeNil())
			})

			It("should set intermediate TLS profile from HCO", func() {
				hco.Spec.Security.TLSSecurityProfile = intermediateTLSSecurityProfile

				cr := NewFileRestoreOperator(hco)

				Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(intermediateTLSSecurityProfile)))
				Expect(cr.Spec.TLSSecurityProfile.Type).To(Equal(vmfr.TLSProfileType(openshiftconfigv1.TLSProfileIntermediateType)))
				Expect(cr.Spec.TLSSecurityProfile.Intermediate).ToNot(BeNil())
				Expect(cr.Spec.TLSSecurityProfile.Custom).To(BeNil())
			})

			It("should set modern TLS profile from HCO", func() {
				hco.Spec.Security.TLSSecurityProfile = modernTLSSecurityProfile

				cr := NewFileRestoreOperator(hco)

				Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(modernTLSSecurityProfile)))
				Expect(cr.Spec.TLSSecurityProfile.Type).To(Equal(vmfr.TLSProfileType(openshiftconfigv1.TLSProfileModernType)))
				Expect(cr.Spec.TLSSecurityProfile.Modern).ToNot(BeNil())
				Expect(cr.Spec.TLSSecurityProfile.Custom).To(BeNil())
			})

			It("should set custom TLS profile from HCO", func() {
				hco.Spec.Security.TLSSecurityProfile = customTLSSecurityProfile

				cr := NewFileRestoreOperator(hco)

				Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(customTLSSecurityProfile)))
				Expect(cr.Spec.TLSSecurityProfile.Type).To(Equal(vmfr.TLSProfileType(openshiftconfigv1.TLSProfileCustomType)))
				Expect(cr.Spec.TLSSecurityProfile.Custom).ToNot(BeNil())
				Expect(cr.Spec.TLSSecurityProfile.Custom.Ciphers).To(Equal([]string{"ECDHE-RSA-AES128-GCM-SHA256"}))
				Expect(cr.Spec.TLSSecurityProfile.Custom.MinTLSVersion).To(Equal(vmfr.TLSProtocolVersion(openshiftconfigv1.VersionTLS12)))
			})

			It("should leave TLSSecurityProfile nil when not set on HCO", func() {
				origFunc := tlssecprofile.GetTLSSecurityProfile
				tlssecprofile.GetTLSSecurityProfile = func(_ *openshiftconfigv1.TLSSecurityProfile) *openshiftconfigv1.TLSSecurityProfile {
					return modernTLSSecurityProfile
				}
				DeferCleanup(func() {
					tlssecprofile.GetTLSSecurityProfile = origFunc
				})

				Expect(hco.Spec.Security.TLSSecurityProfile).To(BeNil())

				cr := NewFileRestoreOperator(hco)

				Expect(cr.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(modernTLSSecurityProfile)))
			})
		})
	})

	Context("check handler Ensure", func() {
		It("should create FileRestoreOperator if it doesn't exist", func() {
			cl = commontestutils.InitClient([]client.Object{hco})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Name).To(Equal("vm-file-restore-" + hcoutil.HyperConvergedName))
			Expect(res.Created).To(BeTrue())
			Expect(res.Updated).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundCR)).To(Succeed())
			Expect(foundCR.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("should update FileRestoreOperator fields if not matched to the requirements", func() {
			existingCR := NewFileRestoreOperatorWithNameOnly()
			existingCR.Spec.ImagePullPolicy = corev1.PullAlways

			cl = commontestutils.InitClient([]client.Object{hco, existingCR})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)

			Expect(res.Err).ToNot(HaveOccurred())
			Expect(res.Created).To(BeFalse())
			Expect(res.Deleted).To(BeFalse())
			Expect(res.Updated).To(BeTrue())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(cl.Get(context.Background(), client.ObjectKey{Name: res.Name, Namespace: hco.Namespace}, foundCR)).To(Succeed())
			Expect(foundCR.Spec.ImagePullPolicy).To(Equal(corev1.PullIfNotPresent))
		})

		It("should reconcile managed labels to default without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"

			outdatedCR := NewFileRestoreOperatorWithNameOnly()
			expectedLabels := maps.Clone(outdatedCR.Labels)
			for k, v := range expectedLabels {
				outdatedCR.Labels[k] = "wrong_" + v
			}
			outdatedCR.Labels[userLabelKey] = userLabelValue

			cl = commontestutils.InitClient([]client.Object{hco, outdatedCR})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedCR.Name, Namespace: outdatedCR.Namespace},
					foundCR),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundCR.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundCR.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})

		It("should reconcile managed labels to default on label deletion without touching user added ones", func() {
			const userLabelKey = "userLabelKey"
			const userLabelValue = "userLabelValue"

			outdatedCR := NewFileRestoreOperatorWithNameOnly()
			expectedLabels := maps.Clone(outdatedCR.Labels)
			outdatedCR.Labels[userLabelKey] = userLabelValue
			delete(outdatedCR.Labels, hcoutil.AppLabelVersion)

			cl = commontestutils.InitClient([]client.Object{hco, outdatedCR})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())

			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: outdatedCR.Name, Namespace: outdatedCR.Namespace},
					foundCR),
			).ToNot(HaveOccurred())

			for k, v := range expectedLabels {
				Expect(foundCR.Labels).To(HaveKeyWithValue(k, v))
			}
			Expect(foundCR.Labels).To(HaveKeyWithValue(userLabelKey, userLabelValue))
		})
	})

	Context("check cache", func() {
		It("should create new cache if empty, return cached object on subsequent calls, and reset clears cache", func() {
			hook := &vmfrHooks{}
			Expect(hook.cache).To(BeNil())

			firstCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(firstCallResult).ToNot(BeNil())
			Expect(hook.cache).To(BeIdenticalTo(firstCallResult))

			secondCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(secondCallResult).ToNot(BeNil())
			Expect(firstCallResult).To(BeIdenticalTo(secondCallResult))

			hook.Reset()
			Expect(hook.cache).To(BeNil())

			thirdCallResult, err := hook.GetFullCr(hco)
			Expect(err).ToNot(HaveOccurred())
			Expect(thirdCallResult).ToNot(BeNil())
			Expect(thirdCallResult).ToNot(BeIdenticalTo(firstCallResult))
		})
	})

	Context("TLSSecurityProfile", func() {
		intermediateTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type:         openshiftconfigv1.TLSProfileIntermediateType,
			Intermediate: &openshiftconfigv1.IntermediateTLSProfile{},
		}
		modernTLSSecurityProfile := &openshiftconfigv1.TLSSecurityProfile{
			Type:   openshiftconfigv1.TLSProfileModernType,
			Modern: &openshiftconfigv1.ModernTLSProfile{},
		}

		It("should update TLSSecurityProfile on FileRestoreOperator CR when HCO changes", func() {
			hco.Spec.Security.TLSSecurityProfile = intermediateTLSSecurityProfile
			existingCR := NewFileRestoreOperator(hco)
			Expect(existingCR.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(intermediateTLSSecurityProfile)))

			hco.Spec.Security.TLSSecurityProfile = modernTLSSecurityProfile

			cl = commontestutils.InitClient([]client.Object{hco, existingCR})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingCR.Name, Namespace: existingCR.Namespace},
					foundCR),
			).ToNot(HaveOccurred())

			Expect(foundCR.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(modernTLSSecurityProfile)))
			Expect(req.Conditions).To(BeEmpty())
		})

		It("should overwrite TLSSecurityProfile if directly set on FileRestoreOperator CR", func() {
			hco.Spec.Security.TLSSecurityProfile = intermediateTLSSecurityProfile
			existingCR := NewFileRestoreOperator(hco)

			req.HCOTriggered = false
			existingCR.Spec.TLSSecurityProfile = openshift2FileRestoreSecProfile(modernTLSSecurityProfile)

			cl = commontestutils.InitClient([]client.Object{hco, existingCR})
			handler := NewVMFileRestoreHandler(cl, commontestutils.GetScheme())
			res := handler.Ensure(req)
			Expect(res.UpgradeDone).To(BeFalse())
			Expect(res.Updated).To(BeTrue())
			Expect(res.Overwritten).To(BeTrue())
			Expect(res.Err).ToNot(HaveOccurred())

			foundCR := &vmfr.FileRestoreOperator{}
			Expect(
				cl.Get(context.TODO(),
					types.NamespacedName{Name: existingCR.Name, Namespace: existingCR.Namespace},
					foundCR),
			).ToNot(HaveOccurred())

			Expect(foundCR.Spec.TLSSecurityProfile).To(Equal(openshift2FileRestoreSecProfile(hco.Spec.Security.TLSSecurityProfile)))
			Expect(foundCR.Spec.TLSSecurityProfile).ToNot(Equal(openshift2FileRestoreSecProfile(modernTLSSecurityProfile)))
			Expect(req.Conditions).To(BeEmpty())
		})
	})
})
